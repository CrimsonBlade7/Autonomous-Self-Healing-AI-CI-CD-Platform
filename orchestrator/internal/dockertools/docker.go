package dockertools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ImageManager interface {
	ImageList(ctx context.Context, options client.ImageListOptions) (client.ImageListResult, error)
	ImageRemove(ctx context.Context, imageID string, options client.ImageRemoveOptions) (client.ImageRemoveResult, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error)
}

type ContainerManager interface {
	ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerLogs(ctx context.Context, containerID string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error)
	ContainerWait(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult
	ContainerInspect(ctx context.Context, containerID string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error)
}

type ContainerInspection struct {
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
	Errors    string
	OOMKilled bool
	Status    string
}

type TarBuilder interface {
	TarWorkspace(pw *io.PipeWriter, path string) error
}

var ImageBuildErr error = errors.New("Image build failed")

// Cleans up old images
func ClearOldImages(ctx context.Context, im ImageManager) (err error) {
	images, err := im.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		return fmt.Errorf("Failed to fetch image list: %w", err)
	}
	for _, item := range images.Items {
		_, err = im.ImageRemove(ctx, item.ID, client.ImageRemoveOptions{})
		if err != nil {
			return fmt.Errorf("Failed to remove image %s: %w", item.ID, err)
		}
	}
	return nil
}

// Builds an image from src with sha as the tag.
func BuildImage(ctx context.Context, im ImageManager, wsName, sha, srcPath string, tb TarBuilder) (tag string, err error) {

	pr, pw := io.Pipe()
	defer func() {
		closeErr := pr.Close()
		if closeErr != nil {
			err = closeErr
		}
	}()
	go func() {
		err := tb.TarWorkspace(pw, srcPath)
		if err != nil {
			slog.Error("Failed to tar the workspace", "error", err)
			return
		}
		err = pw.CloseWithError(err)
		if err != nil {
			slog.Error("Pipe writter failed to close", "error", err)
			return
		}
	}()

	tag = fmt.Sprintf("%s:%s", wsName, sha)

	imageResult, err := im.ImageBuild(ctx, pr, client.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
	})
	if err != nil {
		return "", fmt.Errorf("Failed to build image: %w", err)
	}
	defer func() {
		closeErr := imageResult.Body.Close()
		if closeErr != nil {
			err = closeErr
		}
	}()

	type temp struct {
		ErrorDetail struct {
			Code    int    `json:"code,omitempty"`
			Message string `json:"message,omitempty"`
		} `json:"errorDetail,omitempty"`
	}

	var t temp
	buf, err := io.ReadAll(imageResult.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read image build result: %w", err)
	}
	err = json.Unmarshal(buf, &t)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal image build result: %w", err)
	}

	if t.ErrorDetail.Code != 0 || t.ErrorDetail.Message != "" {
		return "", fmt.Errorf(
			"Failed to build image\n"+
				"Code: %v\n"+
				"Message: %s\n"+
				"Error: %w",
			t.ErrorDetail.Code, t.ErrorDetail.Message, ImageBuildErr)
	}

	return tag, err
}

// Builds and runs a container labeled with tag. Returns the id, stdout, stderr, and an error.
// The caller is responsible for closing the logs.
func RunContainer(ctx context.Context, cm ContainerManager, tag string) (id string, outReader io.ReadCloser, errReader io.ReadCloser, err error) {

	cont, err := cm.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Env: config.TestingEnvSlice,
		},
		HostConfig: &container.HostConfig{
			Resources: container.Resources{
				Memory: int64(config.CONTAINER_MEMORY_CAP * config.MB),
			},
		},
		Name:  tag,
		Image: tag,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("Failed to create container %s: %w", tag, err)
	}
	defer func() {
		_, err = cm.ContainerRemove(ctx, cont.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
		})
		if err != nil {
			slog.Error("Failed to remove container", "error", err)
			return
		}
	}()

	id = cont.ID

	logs, err := cm.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("Failed to create a container logger: %w", err)
	}

	_, err = cm.ContainerStart(ctx, id, client.ContainerStartOptions{})
	if err != nil {
		return "", nil, nil, fmt.Errorf("Failed to start container: %w", err)
	}

	response := cm.ContainerWait(ctx, id, client.ContainerWaitOptions{})
	select {
	case <-response.Result:
		slog.Info("Container completed")
	case err = <-response.Error:
		slog.Error("Container error", "error", err)
	}

	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer func() {
		outWriterErr := outWriter.Close()
		errWriterErr := errWriter.Close()
		closeErr := errors.Join(outWriterErr, errWriterErr)
		if closeErr != nil {
			err = closeErr
		}
	}()
	stdcopy.StdCopy(outWriter, errWriter, logs)

	return id, outReader, errReader, err
}

// Returns current state of a completed container as a ContainerInspection struct.
func InspectContainer(ctx context.Context, cm ContainerManager, id string) (inspection ContainerInspection, err error) {
	cont, err := cm.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("Failed to inspect container %s: %w", id, err)
	}
	contState := cont.Container.State
	start, err := time.Parse(time.RFC3339Nano, contState.StartedAt)
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("Failed to parse start time: %w", err)
	}
	end, err := time.Parse(time.RFC3339Nano, contState.FinishedAt)
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("Failed to parse end time: %w", err)
	}
	inspection = ContainerInspection{
		ExitCode:  contState.ExitCode,
		StartTime: start,
		EndTime:   end,
		Errors:    contState.Error,
		OOMKilled: contState.OOMKilled,
		Status:    string(contState.Status),
	}
	return inspection, err
}

// Removes the specified container.
func RemoveContainer(ctx context.Context, cm ContainerManager, id string) (err error) {
	cont, err := cm.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("Failed to inspect container %s: %w", id, err)
	}
	_, err = cm.ContainerRemove(ctx, cont.Container.ID, client.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Failed to remove container: %w", err)
	}
	return nil
}
