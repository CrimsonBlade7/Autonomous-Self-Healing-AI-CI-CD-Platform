package main

import (
	"context"
	_ "fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/moby/moby/client"
)

// Loads the .env variables
func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}
	secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		return err
	}
	port = os.Getenv("PORT")
	if port == "" {
		return err
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	err := loadEnv()
	if err != nil {
		slog.Error("Failed to load .env variables", "error", err)
		return
	}

	err = cleanBrokenRepos()
	if err != nil {
		slog.Error("Failed to clean broken repos", "error", err)
		return
	}

	mobyClient, err = client.New(client.FromEnv)
	if err != nil {
		slog.Error("Failed to start moby client", "error", err)
		return
	}
	defer mobyClient.Close()

	mainCtx := context.Background()
	err = startJobPipeline(mainCtx)
	if err != nil {
		slog.Error("Failed to start job pipeline", "error", err)
		return
	}

	err = startServer(mainCtx)
	if err != nil {
		slog.Error("Server failure", "error", err)
		return
	}
}

/*
TODO List:
	- fix contexts to have children
	- ...
*/