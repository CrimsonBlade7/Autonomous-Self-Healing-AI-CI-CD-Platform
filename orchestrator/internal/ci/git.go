package ci

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

// Creates a unique temp directory in repoDir
func tempRepoPath(path, sha string) (string, func() error, error) {
	path, err := os.MkdirTemp(path, fmt.Sprintf("sha%s_*", sha))
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return os.RemoveAll(path)
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
func CleanBrokenRepos(path string) error {
	target := "checkout.ready"
	repoDirFiles, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("Failed to read directory %s: %w", path, err)
	}
	for _, repo := range repoDirFiles {
		if repo.IsDir() {
			repoPath := filepath.Join(path, repo.Name())
			path := filepath.Join(repoPath, target)
			_, err = os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				os.RemoveAll(repoPath)
				fmt.Printf("Removed repo: %s\n", repoPath)
			} else if err != nil {
				return fmt.Errorf("Failed to check for .ready file at %s: %w", repoPath, err)
			}
		}
	}
	return nil
}

// Initializes the repository and clones it into path (which should be temp_repos)
// TODO: handle "fetch by sha" disabled
func initializeRepo(path, url, sha string) (string, error) {

	path, cleanup, err := tempRepoPath(path, sha)
	if err != nil {
		return "", fmt.Errorf("Failed to make a temporary directory: %w", err)
	}

	repo, err := git.PlainInit(path, false)
	if err != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to initialize repo %s at %s: %w", url, sha, err)
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	if err != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to create remote %s at %s: %w", url, sha, err)
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteURL: url,
		RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", sha, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to fetch commit %s at %s: %w", url, sha, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to get worktree: %w", err)
	}

	hash, ok := plumbing.FromHex(sha)
	if !ok {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to hash the sha: %w", err)
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to checkout branch: %w", err)
	}

	err = createReadyFile(path)
	if err != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return "", fmt.Errorf("Failed remove the temporary directory %s: %w", path, cleanupErr)
		}
		return "", fmt.Errorf("Failed to create ready file: %w", err)
	}

	slog.Info("Successfully checked out specific SHA natively!")
	return path, nil
}
