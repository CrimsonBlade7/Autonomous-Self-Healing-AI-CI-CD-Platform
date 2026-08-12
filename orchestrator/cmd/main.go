package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/dockertools"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/pipelines"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/servertools"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"

	"github.com/moby/moby/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()
	taskChannel := make(chan types.Task)
	wfm := pipelines.NewWorkflowManager()

	// Initialize environment variables to global scope
	err := config.Init()
	if err != nil {
		slog.Error("Failed to initialize global variables", "error", err)
		return
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		slog.Error("Failed to start moby client", "error", err)
		return
	}
	defer func() {
		err = cli.Close()
		if err != nil {
			slog.Error("Failed to close the docker client", "error", err)
		}
	}()

	err = dockertools.ClearOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go wfm.RunWorkflowPipeline(mainCtx, cli, taskChannel)

	go func() {
		err = servertools.StartServer(mainCtx, taskChannel)
		if err != nil {
			slog.Error("Server failure", "error", err)
			return
		}
	}()
}

/*
TODO List:
	- testing
		- add tests
	- handle receiving prs while workflow is running
		- cancel running containers
		- clear images (optional?)
		- if patches for the wrong sha are received, discard them
	- figure out delivery mechanisms
	- handle hanging workflows due to errors
	- dockerize this project
		- add docker volumes for workspaces
		- also maybe for saving workflows?
	- .env injection *
	- container timeout *
	- container mem cap *
	- log storage

Wishlist
	- multi-service testing
	- seperate test files/test directory structure
	- running tests multiple times?
	- structured test result contracts
	- orchestrator can check the language from a user provided spec?

*/
