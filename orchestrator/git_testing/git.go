package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

var repoDir string = "./temp_repos"

// Creates a unique temp directory in repoDir
func tempRepoPath(sha string) (string, func() error, error) {
	path, err := os.MkdirTemp(repoDir, fmt.Sprintf("commitsha_%s_*", sha))
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

func main() {

	remoteURL := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	commitSHA := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"

	err := cleanBrokenRepos()
	if err != nil {
		panic(fmt.Sprintf("Failed to clean broken repos: %v", err))
	}

	path, cleanup, err := tempRepoPath(commitSHA)
	if err != nil {
		panic(fmt.Sprintf("Failed to create a temporary directory: %s", err))
	}

	repo, err := git.PlainInit(path, false)
	if err != nil {
		cleanup()
		panic(fmt.Sprintf("Failed to initialize repo: %v", err))
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
	})
	if err != nil {
		cleanup()
		panic(fmt.Sprintf("Failed to create remote: %v", err))
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteURL: remoteURL,
		RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", commitSHA, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		cleanup()
		panic(fmt.Sprintf("Failed to fetch commit: %v", err))
	}

	wt, err := repo.Worktree()
	if err != nil {
		cleanup()
		panic(fmt.Sprintf("Failed to get worktree: %v", err))
	}

	hash, ok := plumbing.FromHex(commitSHA)
	if !ok {
		cleanup()
		panic("Failed to hash the commitSHA")
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		cleanup()
		panic(fmt.Sprintf("Failed to checkout branch: %s", err))
	}

	err = createReadyFile(path)
	if err != nil {
		cleanup()
		panic(fmt.Sprintf("Failed to create ready file: %v", err))
	}

	fmt.Println("Successfully checked out specific SHA natively!")
}
