package main

import (
	"context"
	"errors"
	_ "fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/moby/moby/client"
)

var repoDir string = "./temp_repos"

// The Job struct exists in case new fields need to be added
type Job struct {
	PullReq PullRequest
}

// Loads the .env variables
func loadEnv(secret, port *string) error {
	err := godotenv.Load()
	if err != nil {
		return err
	}
	*secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if *secret == "" {
		return errors.New("Secret is empty")
	}
	*port = os.Getenv("PORT")
	if *port == "" {
		return errors.New("Port is empty")
	}
	return nil
}

func main() {
	var secret string
	var port string
	jobs := make(chan Job)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	mainCtx := context.Background()

	err := loadEnv(&secret, &port)
	if err != nil {
		slog.Error("Failed to load .env variables", "error", err)
		return
	}

	err = cleanBrokenRepos()
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

	err = cleanOldImages(mainCtx, cli)
	if err != nil {
		slog.Error("Failed to clean old images", "error", err)
		return
	}

	go startJobPipeline(mainCtx, cli, jobs)

	go func() {
		err = startServer(mainCtx, secret, port, jobs)
		if err != nil {
			slog.Error("Server failure", "error", err)
			return
		}
	}()
}

/*
TODO List:
	- fix contexts to have children
	- create dockerfiles
	- testing
*/
