package wstools

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
)

type GitClient interface {
	InitRepo(ctx context.Context, path string, pr types.PullRequest) error
	CommitPush(commitMsg, wsPath, branch, sha string) error
	UpdateRepo(ctx context.Context, pr types.PullRequest) error
}

// Creates a unique temp directory in workspaceDir
func tempWorkspace(dir, sha string) (string, func() error, error) {
	path, err := os.MkdirTemp(dir, fmt.Sprintf("sha%s_*", sha))
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		cleanupErr := os.RemoveAll(path)
		if cleanupErr != nil {
			return fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return nil
	}
	return path, cleanup, nil
}

// Initializes the workspace and clones it into dest (which should be temp_workspaces)
func InitWorkspace(ctx context.Context, pr types.PullRequest, cli GitClient) (string, func() error, error) {

	path, cleanup, err := tempWorkspace(config.WsDir, pr.HeadSHA)

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
func ClearWorkspaces() error {

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
	if err := os.MkdirAll(config.WsDir, mode); err != nil {
		return fmt.Errorf("Failed to recreate dir: %w", err)
	}

	return nil
}

// TODO: placeholder
func InsertTests() error {
	return nil // stub
}
