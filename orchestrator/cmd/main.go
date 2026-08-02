package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/dockertools"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/pipelines"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/servertools"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/wstools"

	"github.com/moby/moby/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()
	prChannel := make(chan types.PullRequest)

	err := config.Init()
	if err != nil {
		slog.Error("Failed to initialize global variables", "error", err)
		return
	}

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

	err = dockertools.CleanOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go pipelines.StartWorkflowPipeline(mainCtx, cli, prChannel)

	go func() {
		err = servertools.StartServer(mainCtx, prChannel)
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
*/
