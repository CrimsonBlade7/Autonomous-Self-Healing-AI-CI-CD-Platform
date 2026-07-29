package ci

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

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
}

// Cleans up old images
func CleanOldImages(ctx context.Context, im ImageManager) error {
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
func buildImage(ctx context.Context, im ImageManager, wsName, sha, srcPath string) (string, error) {

	pr, pw := io.Pipe()
	defer pr.Close()
	go func() {
		err := tarWorkspace(pw, srcPath)
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

	tag := fmt.Sprintf("%s:%s", wsName, sha)

	imageResult, err := im.ImageBuild(ctx, pr, client.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
	})
	if err != nil {
		return "", fmt.Errorf("Failed to build image: %w", err)
	}
	defer imageResult.Body.Close()

	// TODO: imageResult.Body needs to be drained when complete
	// imageResults is discarded for now
	_, err = io.Copy(io.Discard, imageResult.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to print body: %w", err)
	}

	return tag, nil
}

func tarWorkspace(pw *io.PipeWriter, path string) error {
	tw := tar.NewWriter(pw)
	defer tw.Close()

	err := filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
		relPath, err := filepath.Rel(path, path)
		if err != nil {
			return fmt.Errorf("Failed create relative path %s/%s: %w", path, path, err)
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return fmt.Errorf("Failed to get info for %s: %w", path, err)
		}

		header, err := tar.FileInfoHeader(fi, d.Name())
		if err != nil {
			return fmt.Errorf("Failed to create file info header: %w", err)
		}
		header.Name = filepath.ToSlash(relPath)

		err = tw.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("Failed to write header: %w", err)
		}

		if d.Type().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("Failed to open file %s: %w", path, err)
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("Failed to write file contents to tar writer: %w", err)
			}
		}

		return nil
	})
	return err
}

// Builds and runs a container labeled with tag.
func runContainer(ctx context.Context, cm ContainerManager, tag string) error {

	cont, err := cm.ContainerCreate(ctx, client.ContainerCreateOptions{
		HostConfig: &container.HostConfig{
			AutoRemove: true,
		},
		Name:  tag,
		Image: tag,
	})
	if err != nil {
		return fmt.Errorf("Failed to create container %s: %w", tag, err)
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

	logs, err := cm.ContainerLogs(ctx, cont.ID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("Failed to create a container logger: %w", err)
	}

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	go func() {
		defer logs.Close()
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		_, err = stdcopy.StdCopy(stdoutWriter, stderrWriter, logs)
		if err != nil {
			slog.Error("Failed reading logs", "error", err)
			return
		}
	}()

	go func() {
		defer stdoutReader.Close()
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			slog.Info(scanner.Text(), "container_id", cont.ID)
		}
		err = scanner.Err()
		if err != nil {
			slog.Error("Failed to scan the logs", "error", err)
			return
		}
	}()

	go func() {
		defer stderrReader.Close()
		scanner := bufio.NewScanner(stderrReader)
		for scanner.Scan() {
			slog.Error(scanner.Text(), "container_id", cont.ID)
		}
		err = scanner.Err()
		if err != nil {
			slog.Error("Failed to scan the logs", "error", err)
			return
		}
	}()

	_, err = cm.ContainerStart(ctx, cont.ID, client.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("Failed to start container: %w", err)
	}

	response := cm.ContainerWait(ctx, cont.ID, client.ContainerWaitOptions{})
	select {
	case <-response.Result:
		slog.Info("Container completed")
	case err = <-response.Error:
		slog.Error("Container error", "error", err)
	}

	bytes, err := io.ReadAll(logs)
	if err != nil {
		return fmt.Errorf("Failed to read logs: %w", err)
	}

	slog.Info("Container Logs", slog.String("logs", string(bytes)))

	return nil
}
