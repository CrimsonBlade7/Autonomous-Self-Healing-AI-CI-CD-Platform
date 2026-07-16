package main

import (
	"fmt"
	"log"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

func main() {
	// Define configuration variables
	remoteURL := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	commitSHA := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71" // Replace with your SHA
	directory := "./test_project_folder"

	// Simulate 'git init' inside the target directory
	repo, err := git.PlainOpen(directory)
	if err == git.ErrRepositoryNotExists {
		repo, err = git.PlainInit(directory, false)
		if err != nil {
			log.Fatalf("Failed to initialize the repository: %v", err)
		}
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteURL},
		})
		if err != nil {
			log.Fatalf("Failed to add remote: %v", err)
		}
	} else if err != nil {
		log.Fatalf("Failed to open the repository: %v", err)
	}

	// Simulate 'git fetch origin <sha>'
	// We pass a custom Refspec to fetch only the explicit commit hash
	err = repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("%s:%s", commitSHA, "/refs/heads/temp-branch")),
		},
		Depth: 1,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Fatalf("Failed to fetch specific SHA: %v", err)
	}

	// Simulate 'git checkout <sha>'
	w, err := repo.Worktree()
	if err != nil {
		log.Fatalf("Failed to get worktree: %v", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(commitSHA),
		Force: true,
	})
	if err != nil {
		log.Fatalf("Failed to checkout SHA: %v", err)
	}

	fmt.Println("Successfully checked out specific SHA natively!")
}
