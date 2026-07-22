package main

import (
	"context"
	"log/slog"
)

func startJobPipeline(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-jobQueue:
			err := initializeRepo(job.PullReq.Url, job.PullReq.HeadSHA)
			if err != nil {
				slog.Error("Failed to initialize the repository", "error", err)
			}
			// TODO: create a docker container
		}
	}
}

/*
	url := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	sha := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
*/
