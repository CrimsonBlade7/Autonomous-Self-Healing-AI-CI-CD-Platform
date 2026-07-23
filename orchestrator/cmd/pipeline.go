package main

import (
	"context"
	"log/slog"

	"github.com/moby/moby/client"
)

func startJobPipeline(ctx context.Context, cli *client.Client, jobs chan Job) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			path, err := initializeRepo(job.PullReq.Url, job.PullReq.HeadSHA)
			if err != nil {
				slog.Error("Failed to initialize the repository", "error", err)
				continue
			}
			err = buildImage(ctx, cli, job.PullReq.HeadSHA, path)
			if err != nil {
				slog.Error("Failed to build image", "error", err)
				continue
			}
			err = runContainer(ctx, cli, path)
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
