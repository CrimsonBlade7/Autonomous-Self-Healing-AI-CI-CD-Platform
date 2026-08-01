package ci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
)

type GitClient interface {
	CheckoutCommit(ctx context.Context, path string, pr types.PullRequest) error
}

// Creates a unique temp directory in workspaceDir
func tempWorkspacePath(path, sha string) (string, func() error, error) {
	path, err := os.MkdirTemp(path, fmt.Sprintf("sha%s_*", sha))
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

// Creates a ready file in the directory at the specified path
func createReadyFile(path string) error {
	file, err := os.OpenFile(fmt.Sprintf("%s/checkout.ready", path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Failed to create ready file at %s: %w", path, err)
	}
	return file.Close()
}

// Removes directories in path that do not contain a checkout.ready file
func CleanBrokenWorkspaces(path string) error {
	readyFilename := "checkout.ready"
	wsDirFiles, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("Failed to read directory %s: %w", path, err)
	}
	for _, ws := range wsDirFiles {
		if ws.IsDir() {
			wsPath := filepath.Join(path, ws.Name())
			readyPath := filepath.Join(wsPath, readyFilename)
			_, err = os.Stat(readyPath)
			if errors.Is(err, os.ErrNotExist) {
				os.RemoveAll(wsPath)
				fmt.Printf("Removed workspace: %s\n", wsPath)
			} else if err != nil {
				return fmt.Errorf("Failed to check for .ready file at %s: %w", wsPath, err)
			}
		}
	}
	return nil
}

// Initializes the workspace and clones it into dest (which should be temp_workspaces)
func initializeWorkspace(ctx context.Context, dest string, pr types.PullRequest, cli GitClient) (string, error) {

	path, cleanup, err := tempWorkspacePath(dest, pr.HeadSHA)
	if err != nil {
		return "", fmt.Errorf("Failed to make a temporary directory: %w", err)
	}

	err = cli.CheckoutCommit(ctx, path, pr)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("Failed to checkout commit: %v", err)
	}

	err = createReadyFile(path)
	if err != nil {
		err = cleanup()
		return "", fmt.Errorf("Failed to create ready file: %w", err)
	}

	slog.Info("Successfully checked out specific SHA natively!")
	return path, nil
}

// TODO: placeholder
func updateWorkspace() error {
	return errors.New("Unimplemented function") // stub
}

// TODO: placeholder
func insertTests() error {
	return errors.New("Unimplemented function") // stub
}
