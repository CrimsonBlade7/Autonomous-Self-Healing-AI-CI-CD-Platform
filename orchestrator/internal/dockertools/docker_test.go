package dockertools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	dockerClient "github.com/moby/moby/client"
)

type fakeImageManager struct {
	list    dockerClient.ImageListResult
	listErr error
	removed []string
	removeErr error
	build   dockerClient.ImageBuildResult
	buildErr error
}

func (f *fakeImageManager) ImageList(ctx context.Context, options dockerClient.ImageListOptions) (dockerClient.ImageListResult, error) {
	return f.list, f.listErr
}

func (f *fakeImageManager) ImageRemove(ctx context.Context, tag string, options dockerClient.ImageRemoveOptions) (dockerClient.ImageRemoveResult, error) {
	f.removed = append(f.removed, tag)
	return dockerClient.ImageRemoveResult{}, f.removeErr
}

func (f *fakeImageManager) ImageBuild(ctx context.Context, buildContext io.Reader, options dockerClient.ImageBuildOptions) (dockerClient.ImageBuildResult, error) {
	_, _ = io.Copy(io.Discard, buildContext)
	return f.build, f.buildErr
}

type fakeContainerManager struct {
	list       dockerClient.ContainerListResult
	listErr    error
	removed    []string
	removeErr  error
	create     dockerClient.ContainerCreateResult
	createErr  error
	logs       dockerClient.ContainerLogsResult
	logsErr    error
	startErr   error
	wait       dockerClient.ContainerWaitResult
	inspect    dockerClient.ContainerInspectResult
	inspectErr error
}

func (f *fakeContainerManager) ContainerList(ctx context.Context, options dockerClient.ContainerListOptions) (dockerClient.ContainerListResult, error) {
	return f.list, f.listErr
}

func (f *fakeContainerManager) ContainerCreate(ctx context.Context, options dockerClient.ContainerCreateOptions) (dockerClient.ContainerCreateResult, error) {
	return f.create, f.createErr
}

func (f *fakeContainerManager) ContainerRemove(ctx context.Context, containerID string, options dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error) {
	f.removed = append(f.removed, containerID)
	return dockerClient.ContainerRemoveResult{}, f.removeErr
}

func (f *fakeContainerManager) ContainerLogs(ctx context.Context, containerID string, options dockerClient.ContainerLogsOptions) (dockerClient.ContainerLogsResult, error) {
	return f.logs, f.logsErr
}

func (f *fakeContainerManager) ContainerStart(ctx context.Context, containerID string, options dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
	return dockerClient.ContainerStartResult{}, f.startErr
}

func (f *fakeContainerManager) ContainerWait(ctx context.Context, containerID string, options dockerClient.ContainerWaitOptions) dockerClient.ContainerWaitResult {
	return f.wait
}

func (f *fakeContainerManager) ContainerInspect(ctx context.Context, containerID string, options dockerClient.ContainerInspectOptions) (dockerClient.ContainerInspectResult, error) {
	return f.inspect, f.inspectErr
}

type nopTarBuilder struct{}

func (nopTarBuilder) TarWorkspace(pw *io.PipeWriter, path string) error {
	return nil
}

func TestClearOldImages(t *testing.T) {
	im := &fakeImageManager{
		list: dockerClient.ImageListResult{Items: []image.Summary{{ID: "img1"}, {ID: "img2"}}},
	}
	if err := ClearOldImages(context.Background(), im); err != nil {
		t.Fatal(err)
	}
	if len(im.removed) != 2 {
		t.Errorf("removed = %v", im.removed)
	}
}

func TestClearOldImages_ListError(t *testing.T) {
	im := &fakeImageManager{listErr: errors.New("boom")}
	if err := ClearOldImages(context.Background(), im); err == nil {
		t.Fatal("expected error")
	}
}

func TestClearOldContainers(t *testing.T) {
	cm := &fakeContainerManager{
		list: dockerClient.ContainerListResult{Items: []container.Summary{{ID: "c1"}}},
	}
	if err := ClearOldContainers(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	if len(cm.removed) != 1 || cm.removed[0] != "c1" {
		t.Errorf("removed = %v", cm.removed)
	}
}

func TestBuildImage_Success(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{"stream":"ok"}`))
	im := &fakeImageManager{build: dockerClient.ImageBuildResult{Body: body}}
	tag, err := BuildImage(context.Background(), im, "ws", "sha1", ".", nopTarBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	if tag != "ws:sha1" {
		t.Errorf("tag = %q", tag)
	}
}

func TestBuildImage_ErrorDetail(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"errorDetail": map[string]any{"code": 1, "message": "fail"},
	})
	im := &fakeImageManager{build: dockerClient.ImageBuildResult{Body: io.NopCloser(bytes.NewReader(payload))}}
	_, err := BuildImage(context.Background(), im, "ws", "sha1", ".", nopTarBuilder{})
	if err == nil {
		t.Fatal("expected build error")
	}
	if !errors.Is(err, ImageBuildErr) {
		t.Errorf("expected ImageBuildErr, got %v", err)
	}
}

func TestRemoveImage(t *testing.T) {
	im := &fakeImageManager{}
	if err := RemoveImage(context.Background(), im, "tag"); err != nil {
		t.Fatal(err)
	}
	if len(im.removed) != 1 {
		t.Errorf("removed = %v", im.removed)
	}
}

func TestInspectContainer(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	end := start.Add(time.Second)
	cm := &fakeContainerManager{
		inspect: dockerClient.ContainerInspectResult{
			Container: container.InspectResponse{
				State: &container.State{
					ExitCode:   7,
					StartedAt:  start.Format(time.RFC3339Nano),
					FinishedAt: end.Format(time.RFC3339Nano),
					Error:      "nope",
					OOMKilled:  true,
					Status:     container.StateExited,
				},
			},
		},
	}
	got, err := InspectContainer(context.Background(), cm, "cid")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 7 || !got.OOMKilled || got.Errors != "nope" {
		t.Errorf("inspection = %+v", got)
	}
	if !got.StartTime.Equal(start) || !got.EndTime.Equal(end) {
		t.Errorf("times start=%v end=%v", got.StartTime, got.EndTime)
	}
}

func TestRemoveContainer(t *testing.T) {
	cm := &fakeContainerManager{
		inspect: dockerClient.ContainerInspectResult{
			Container: container.InspectResponse{ID: "cid"},
		},
	}
	if err := RemoveContainer(context.Background(), cm, "cid"); err != nil {
		t.Fatal(err)
	}
	if len(cm.removed) != 1 || cm.removed[0] != "cid" {
		t.Errorf("removed = %v", cm.removed)
	}
}

func TestRunContainer_CreateError(t *testing.T) {
	cm := &fakeContainerManager{createErr: errors.New("no docker")}
	_, _, _, err := RunContainer(context.Background(), cm, "tag")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunContainer_WaitError(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- errors.New("wait failed")
	cm := &fakeContainerManager{
		create: dockerClient.ContainerCreateResult{ID: "cid"},
		logs:   io.NopCloser(strings.NewReader("")),
		wait: dockerClient.ContainerWaitResult{
			Result: make(chan container.WaitResponse),
			Error:  errCh,
		},
	}
	_, _, _, err := RunContainer(context.Background(), cm, "tag")
	if err == nil {
		t.Fatal("expected wait error")
	}
}

func TestRunContainer_Success(t *testing.T) {
	resCh := make(chan container.WaitResponse, 1)
	resCh <- container.WaitResponse{}
	cm := &fakeContainerManager{
		create: dockerClient.ContainerCreateResult{ID: "cid"},
		logs:   io.NopCloser(strings.NewReader("")),
		wait: dockerClient.ContainerWaitResult{
			Result: resCh,
			Error:  make(chan error),
		},
	}
	id, outR, errR, err := RunContainer(context.Background(), cm, "tag")
	if err != nil {
		t.Fatal(err)
	}
	if id != "cid" {
		t.Errorf("id = %q", id)
	}
	_, _ = io.Copy(io.Discard, outR)
	_, _ = io.Copy(io.Discard, errR)
	_ = outR.Close()
	_ = errR.Close()
}
