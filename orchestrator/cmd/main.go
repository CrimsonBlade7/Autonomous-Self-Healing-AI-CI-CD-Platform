package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/config"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/dockertools"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/pipelines"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/servertools"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"

	dockerClient "github.com/moby/moby/client"
)

func main() {
	// slogs are currently just printed to stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()
	taskChannel := make(chan types.Task)
	pcMap := types.NewPushedCommits()
	wfm := pipelines.NewWorkflowManager()

	// Initialize environment variables to global scope
	if err := config.Init(); err != nil {
		slog.Error("Failed to initialize global variables", "error", err)
		return
	}

	cli, err := dockerClient.New(dockerClient.FromEnv)
	if err != nil {
		slog.Error("Failed to start moby client", "error", err)
		return
	}
	defer func() {
		if err := cli.Close(); err != nil {
			slog.Error("Failed to close the docker client", "error", err)
		}
	}()

	if err := dockertools.ClearOldImages(mainCtx, cli); err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	if err := dockertools.ClearOldContainers(mainCtx, cli); err != nil {
		slog.Error("Failed to clean old containers", "error", err)
		return
	}

	go wfm.RunWorkflowPipeline(mainCtx, cli, taskChannel, pcMap)

	if err := servertools.StartServer(mainCtx, taskChannel, pcMap); err != nil {
		slog.Error("Server failure", "error", err)
		return
	}
}

/*
TODO List:
	- testing *
	- handle hanging workflows due to errors: currently we just drop the whole workflow with no retry
	- dockerize this project
	- add docker volumes
		- workspaces
		- pushed commits
		- workflows
		- log storage
	- handle dead containers
	- fix .env overriding GB multiplier *

Wishlist
	- multi-service testing
	- seperate test files/test directory structure
	- running tests multiple times?
	- structured test result contracts
	- orchestrator can check the language from a user provided spec?

*/
