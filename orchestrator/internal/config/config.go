package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// TODO: figure out which should be in the .env
var RootDir string
var WsDir string
var RepositoryUrl string
var GithubSecret string
var Port string
var BotName string = "ci-cd-bot"
var AIServiceUrl string
var AIServiceSecret string
var ServerShutdownTimeLimit uint = 30

/*
	url := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	sha := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
*/

// Loads the env variables
func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("Failed to load .env file: %w", err)
	}
	RepositoryUrl = os.Getenv("GITHUB_REPOSITORY_URL")
	if RepositoryUrl == "" {
		return fmt.Errorf("Repository url is empty")
	}
	GithubSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if GithubSecret == "" {
		return fmt.Errorf("Secret is empty")
	}
	AIServiceUrl = os.Getenv("AI_SERVICE_URL")
	if AIServiceUrl == "" {
		return fmt.Errorf("AI service url is empty")
	}
	AIServiceSecret = os.Getenv("AI_SERVICE_URL")
	if AIServiceSecret == "" {
		return fmt.Errorf("AI service secret is empty")
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
