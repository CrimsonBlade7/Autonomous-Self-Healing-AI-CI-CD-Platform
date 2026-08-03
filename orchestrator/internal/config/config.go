package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var RootDir string
var WsDir string
var Secret string
var Port string
var BotName string = "ci-cd-bot"
var ServerShutdownTimeLimit uint = 30

// Loads the env variables
func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("Failed to load .env file: %w", err)
	}
	Secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if Secret == "" {
		return fmt.Errorf("Secret is empty")
	}
	Port = os.Getenv("PORT")
	if Port == "" {
		return fmt.Errorf("Port is empty")
	}
	return nil
}

// Initializes global variables
func Init() error {
	err := loadEnv()
	if err != nil {
		return fmt.Errorf("Failed to load env variables: %w", err)
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get executable path: %w", err)
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("Failed to resolve symlinks: %w", err)
	}

	// Set the global BaseDir to the executable's directory

	// Warning: only works if the path to main is orchestrator/cmd/main.go
	RootDir = filepath.Dir(filepath.Dir(exePath))
	WsDir = GetPath("/temp_workspaces")
	return nil
}

// GetPath returns the relative path from the root orchestrator (RootDir + relPath)
func GetPath(relPath string) string {
	return filepath.Join(RootDir, relPath)
}
