package main

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
func tempRepoPath(sha string) (string, func() error, error) {
	path, err := os.MkdirTemp(repoDir, fmt.Sprintf("sha%s_*", sha))
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return os.RemoveAll(path)
	}
	return path, cleanup, err
}

// Creates a ready file in the directory at the specified path
func createReadyFile(path string) error {
	file, err := os.OpenFile(fmt.Sprintf("%s/checkout.ready", path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// Removes directories in repoDir that do not contain a checkout.ready file
func cleanBrokenRepos() error {
	target := "checkout.ready"
	repoDirFiles, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}
	for _, repo := range repoDirFiles {
		if repo.IsDir() {
			repoPath := filepath.Join(repoDir, repo.Name())
			path := filepath.Join(repoPath, target)
			_, err = os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				os.RemoveAll(repoPath)
				fmt.Printf("Removed repo: %s\n", repoPath)
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}

func initializeRepo(url, sha string) error {

	path, cleanup, err := tempRepoPath(sha)
	if err != nil {
		return err
	}

	repo, err := git.PlainInit(path, false)
	if err != nil {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to initialize repo")
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	if err != nil {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to create remote")
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteURL: url,
		RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", sha, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to fetch commit")
	}

	wt, err := repo.Worktree()
	if err != nil {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to get worktree")
	}

	hash, ok := plumbing.FromHex(sha)
	if !ok {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to hash the sha")
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to checkout branch")
	}

	err = createReadyFile(path)
	if err != nil {
		err = cleanup()
		if err != nil {
			return err
		}
		slog.Error("Failed to create ready file")
	}

	fmt.Println("Successfully checked out specific SHA natively!")
	return nil
}
