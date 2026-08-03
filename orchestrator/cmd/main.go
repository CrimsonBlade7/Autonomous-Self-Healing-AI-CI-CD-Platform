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
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/wstools"

	"github.com/moby/moby/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()
	prChannel := make(chan types.PullRequest)
	// patchChannel := make(chan types.PullRequest)
	wfm := pipelines.NewWorkflowManager()

	// Initialize environment variables to global scope
	err := config.Init()
	if err != nil {
		slog.Error("Failed to initialize global variables", "error", err)
		return
	}

	// TODO: temp in case server crashes
	err = wstools.ClearWorkspaces()
	if err != nil {
		slog.Error("Failed to clean broken workspaces", "error", err)
		return
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		slog.Error("Failed to start moby client", "error", err)
		return
	}
	defer cli.Close()

	err = dockertools.ClearOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go wfm.RunWorkflowPipeline(mainCtx, cli, prChannel)

	go func() {
		// TODO: add patch channel
		err = servertools.StartServer(mainCtx, prChannel, nil)
		if err != nil {
			slog.Error("Server failure", "error", err)
			return
		}
	}()
}

/*
TODO List:
	- testing
		- add tests for the rest of the functions other than pr
	- remove images and containers on success, keep of failure for inspection
	- create an error channel?
	- do not create a new request if the new commit came from this service
	- pr.url is not necessary; add it to config
	- handle receiving prs while workflow is running
		- cancel running containers
		- clear images (optional?)
		- update workspace
		- if patches for the wrong sha are received, discard them
*/
