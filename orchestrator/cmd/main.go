package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/ci"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/joho/godotenv"
	"github.com/moby/moby/client"
)

var repoDir string = "./temp_repos"

// Loads the .env variables
func loadEnv(secret, port *string) error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("Failed to load .env file: %w", err)
	}
	*secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if *secret == "" {
		return fmt.Errorf("Secret is empty")
	}
	*port = os.Getenv("PORT")
	if *port == "" {
		return fmt.Errorf("Port is empty")
	}
	return nil
}

func main() {
	var secret string
	var port string
	jobs := make(chan types.Job)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()

	err := loadEnv(&secret, &port)
	if err != nil {
		slog.Error("Failed to load .env variables", "error", err)
		return
	}

	err = ci.CleanBrokenRepos(repoDir)
	if err != nil {
		slog.Error("Failed to clean broken repos", "error", err)
		return
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		slog.Error("Failed to start moby client", "error", err)
		return
	}
	defer cli.Close()

	err = ci.CleanOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go ci.StartJobPipeline(mainCtx, repoDir, cli, jobs)

	go func() {
		err = ci.StartServer(mainCtx, secret, port, jobs)
		if err != nil {
			slog.Error("Server failure", "error", err)
			return
		}
	}()
}

/*
TODO List:
	- testing
		- interfaces for swapping out tests
		- add tests for the rest of the functions other than pr
	- remove images and containers on success, keep of failure for inspection
	- create an error channel?
	- consider fast moving branches where commits are pushed after the webhook fires when handling "fetch by ref"
*/
