package ci

import (
	"context"
	"log/slog"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
)

// Starts the job pipeline.
// Handles incoming jobs.
func StartJobPipeline(ctx context.Context, wsDir string, cli *client.Client, jobs chan types.Job) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			path, err := initializeWorkspace(ctx, wsDir, job.PullReq, &GithubClient{})
			if err != nil {
				slog.Error("Failed to initialize the workspace", "error", err)
				continue
			}

			// TODO: insert tests from rag pipeline

			err = insertTests()
			if err != nil {
				slog.Error("Failed to insert tests", "error", err)
				continue
			}
			tag, err := buildImage(ctx, cli, job.PullReq.Name, job.PullReq.HeadSHA, path, &RealTarBuilder{})
			if err != nil {
				slog.Error("Failed to build image", "error", err)
				continue
			}
			err = runContainer(ctx, cli, tag)
			if err != nil {
				slog.Error("Failed to build container", "error", err)
				continue
			}
		}
	}
}

/*
	url := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	sha := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
*/
