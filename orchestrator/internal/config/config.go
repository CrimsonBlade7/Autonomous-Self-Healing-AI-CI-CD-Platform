package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var Port string = "8080"
var RootDir string // The root of this project.
var RepoDir string // The relative directory that contains the repository root.
var WsDir string   // The relative directory that contains all workspaces.
var GithubToken string
var RepositoryUrl string
var GithubSecret string
var GithubBotLogin string
var AIEngineSecret string
var AIEnginePort string
var AIEngineRequestTimeout int = 5
var ServerShutdownTimeout int = 30
var ReadHeaderTimeout int = 2
var WriteTimeout int = 5
var MaxTestPatchingAttempts int = 10

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
	GithubToken = os.Getenv("GITHUB_TOKEN")
	if GithubToken == "" {
		return fmt.Errorf("Github token is empty.")
	}
	RepositoryUrl = os.Getenv("GITHUB_REPOSITORY_URL")
	if RepositoryUrl == "" {
		return fmt.Errorf("Repository url is empty.")
	}
	GithubSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	if GithubSecret == "" {
		return fmt.Errorf("Github secret is empty.")
	}
	GithubBotLogin = os.Getenv("GITHUB_BOT_LOGIN")
	if GithubBotLogin == "" {
		return fmt.Errorf("Github bot login is empty.")
	}
	Port = os.Getenv("PORT")
	if Port == "" {
		return fmt.Errorf("Port is empty.")
	}
	AIEngineSecret = os.Getenv("AI_ENGINE_SECRET")
	if AIEngineSecret == "" {
		return fmt.Errorf("AI engine secret is empty.")
	}
	AIEnginePort = os.Getenv("AI_ENGINE_PORT")
	if AIEnginePort == "" {
		return fmt.Errorf("AI engine port is empty.")
	}

	return nil
}

// Initializes global variables. Must be called first in main.
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
	// RootDir = ./Autonomous-AI-CI-CD-Platform
	RootDir = filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
	WsDir = GetPath("/shared/workspaces")
	RepoDir = GetPath("/shared")
	return nil
}

// GetPath returns the relative path from the root orchestrator (RootDir + relPath)
func GetPath(relPath string) string {
	return filepath.Join(RootDir, relPath)
}
