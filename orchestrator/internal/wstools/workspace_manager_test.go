package wstools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/config"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"
)

type stubGitClient struct {
	initErr error
	pushSHA string
	pushErr error
	inited  string
}

func (s *stubGitClient) InitRepo(ctx context.Context, path string, pr types.PullRequest) error {
	s.inited = path
	return s.initErr
}

func (s *stubGitClient) AddAllCommitPush(commitMsg, wsPath, branch string) (string, error) {
	return s.pushSHA, s.pushErr
}

func TestInsertTestsAndWriteSummary(t *testing.T) {
	dir := t.TempDir()
	testPath := filepath.Join(dir, "foo_test.go")
	if err := InsertTests(testPath, []byte("package foo")); err != nil {
		t.Fatalf("InsertTests: %v", err)
	}
	got, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package foo" {
		t.Errorf("tests file = %q", got)
	}
}

func TestInitWorkspace_Success(t *testing.T) {
	prev := config.WsDir
	t.Cleanup(func() { config.WsDir = prev })
	config.WsDir = t.TempDir()

	cli := &stubGitClient{}
	path, cleanup, err := InitWorkspace(context.Background(), types.PullRequest{HeadSHA: "abc"}, cli)
	if err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })
	if path == "" || cli.inited != path {
		t.Errorf("path=%q inited=%q", path, cli.inited)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("workspace missing: %v", err)
	}
}

func TestInitWorkspace_InitFailureCleansUp(t *testing.T) {
	prev := config.WsDir
	t.Cleanup(func() { config.WsDir = prev })
	config.WsDir = t.TempDir()

	cli := &stubGitClient{initErr: errors.New("clone failed")}
	path, cleanup, err := InitWorkspace(context.Background(), types.PullRequest{HeadSHA: "abc"}, cli)
	if err == nil {
		t.Fatal("expected error")
	}
	if cleanup != nil {
		t.Fatal("cleanup should be nil after failed init")
	}
	if path != "" {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("workspace %s should have been removed", path)
		}
	}
}

func TestClearWorkspaces(t *testing.T) {
	prev := config.WsDir
	t.Cleanup(func() { config.WsDir = prev })
	dir := t.TempDir()
	config.WsDir = filepath.Join(dir, "workspaces")
	if err := os.MkdirAll(filepath.Join(config.WsDir, "leftover"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ClearWorkspaces(); err != nil {
		t.Fatalf("ClearWorkspaces: %v", err)
	}
	info, err := os.Stat(config.WsDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected recreated directory")
	}
	entries, err := os.ReadDir(config.WsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %v", entries)
	}
}
