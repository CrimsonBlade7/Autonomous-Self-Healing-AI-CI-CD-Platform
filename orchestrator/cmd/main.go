package main

import (
	"fmt"
	"os"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/utils"

	"github.com/joho/godotenv"
)

var (
	secret string
	port   string
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
	loadEnv()

}
