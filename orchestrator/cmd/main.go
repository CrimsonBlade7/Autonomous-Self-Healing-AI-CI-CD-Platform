package main

import (
	"context"
	"log/slog"
	"os"
	"time"

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
	prChan := make(chan types.PullRequest)
	aierChan := make(chan types.AIEngineResponse)
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

	if err := dockertools.WaitForDocker(mainCtx, cli, 10*time.Second); err != nil {
		slog.Error("Docker daemon unreachable", "error", err)
		return
	}

	if err := dockertools.ClearOldContainers(mainCtx, cli); err != nil {
		slog.Error("Failed to clean old containers", "error", err)
		return
	}

	if err := dockertools.ClearOldImages(mainCtx, cli); err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go wfm.RunWorkflowPipeline(mainCtx, cli, prChan, aierChan, pcMap)

	if err := servertools.StartServer(mainCtx, prChan, aierChan, pcMap); err != nil {
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
	- list testing and platform limitations and conditions; close platfrom if conditions are not met *

Wishlist
	- multi-service testing
	- seperate test files/test directory structure
	- running tests multiple times?
	- structured test result contracts
	- orchestrator can check the language from a user provided spec?

*/
