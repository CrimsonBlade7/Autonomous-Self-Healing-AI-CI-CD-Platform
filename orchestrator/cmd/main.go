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
	prChannel := make(chan types.PullRequest)
	respChannel := make(chan types.Response)
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
	defer cli.Close()

	err = dockertools.ClearOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go wfm.RunWorkflowPipeline(mainCtx, cli, prChannel, respChannel)

	go func() {
		// TODO: add patch channel
		err = servertools.StartServer(mainCtx, prChannel, respChannel)
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
	- handle receiving prs while workflow is running
		- cancel running containers
		- clear images (optional?)
		- update workspace
		- if patches for the wrong sha are received, discard them
	- handle max test patching attempts
	- * make whole repository aware
		- change webhook to trigger on push to main
		- handle updates to main
		- save the whole repo
		- make a snapshot at a pr
		- if a push arrives during a workflow, scrap the current job using context cancel on sync
	- figure out delivery mechanisms
		- tests: http body
		- logs: http body
		- repo: path + id + etc and shared volume for persistance
*/
