package wstools

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
)

type GitClient interface {
	InitRepo(ctx context.Context, path string, pr types.PullRequest) error
	CommitPush(commitMsg, wsPath, branch, sha string) error
	UpdateWorkspace(ctx context.Context, wsPath string, pr types.PullRequest) error
}

// Creates a unique temp directory in workspaceDir
func tempWorkspace(dir, sha string) (path string, cleanup func() error, err error) {
	path, err = os.MkdirTemp(dir, fmt.Sprintf("sha%s_*", sha))
	if err != nil {
		return "", nil, err
	}
	cleanup = func() error {
		cleanupErr := os.RemoveAll(path)
		if cleanupErr != nil {
			return fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return nil
	}
	return path, cleanup, nil
}

// Initializes the workspace and clones it into dest (which should be temp_workspaces)
func InitWorkspace(ctx context.Context, pr types.PullRequest, cli GitClient) (path string, cleanup func() error, err error) {

	path, cleanup, err = tempWorkspace(config.WsDir, pr.HeadSHA)

	err = cli.InitRepo(ctx, path, pr)
	if err != nil {
		cleanerr := cleanup()
		if cleanerr != nil {
			return "", nil, fmt.Errorf("Failed to clean up workspace: %w", cleanerr)
		}
		return "", nil, fmt.Errorf("Failed to checkout commit: %v", err)
	}

	slog.Info("Successfully checked out specific SHA natively!")
	return path, cleanup, nil
}

// Clears the temp_workspace directory
func ClearWorkspaces() (err error) {

	// Get original folder permissions to recreate it accurately (optional)
	info, err := os.Stat(config.WsDir)
	mode := os.FileMode(0755)
	if err == nil {
		mode = info.Mode().Perm()
	}

	// Remove everything including the root dir
	if err := os.RemoveAll(config.WsDir); err != nil {
		return fmt.Errorf("Failed to remove dir: %w", err)
	}

	// Recreate the empty directory with original permissions
	if err := os.MkdirAll(config.GetPath(config.WsDir), mode); err != nil {
		return fmt.Errorf("Failed to recreate dir: %w", err)
	}

	return nil
}

// Parses and inserts tests. If all is true, all tests will be selected. Otherwise, all tests in testNames will be selected,
// If a test name does not exist, it will be logged, but no errors will be returned.
func InsertTests(path string, data []byte) (err error) {
	file, err := os.Create(config.GetPath(path))
	if err != nil {
		return fmt.Errorf("Failed to create test file: %w", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write tests: %w", err)
	}
	
	return nil
}
