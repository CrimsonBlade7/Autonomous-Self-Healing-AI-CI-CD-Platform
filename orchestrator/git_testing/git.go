package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

var repoDir string = "./temp_repos"

func tempRepoPath(sha string) (string, error) {
	return os.MkdirTemp(repoDir, fmt.Sprintf("commitsha_%s_*", sha))
}

func main() {

	remoteURL := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	commitSHA := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
	
	dir, err := tempRepoPath(commitSHA)
	if err != nil {
		log.Fatalf("Failed to create a temporary directory: %s", err)
	}
	repo, err := git.PlainOpen(dir)
	if err == git.ErrRepositoryNotExists {
		repo, err = git.PlainInit(dir, false)
		if err != nil {
			log.Fatalf("Failed to initialize repo: %v", err)
		}
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteURL},
		})
		if err != nil {
			log.Fatalf("Failed to create remote: %v", err)
		}
	} else {
		if err != nil {
			log.Fatalf("Failed to open repo: %v", err)
		}
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteURL: remoteURL,
		RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", commitSHA, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Fatalf("Failed to fetch commit: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		log.Fatalf("Failed to get worktree: %v", err)
	}

	hash, ok := plumbing.FromHex(commitSHA)
	if !ok {
		log.Fatalf("Failed to hash the commitSHA")
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		log.Fatalf("Failed to checkout branch: %s", err)
	}
	fmt.Println("Successfully checked out specific SHA natively!")
}
