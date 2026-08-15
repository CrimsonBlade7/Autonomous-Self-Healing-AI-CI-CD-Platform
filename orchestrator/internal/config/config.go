package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

var Port string = "8080"
var RootDir string // The root of this project.
var WsDir string   // The relative directory that contains all workspaces.
var GithubToken string
var RepositoryUrl string
var GithubSecret string
var AIEngineSecret string
var AIEnginePort string = "8000"
var TestingEnvSlice []string

const (
	BYTE int = 1
	KB   int = 1e3 * BYTE
	MB   int = 1e3 * KB
	GB   int = 1e3 * MB

	AI_ENGINE_REQUEST_TIMEOUT  int = 5
	SERVER_SHUTDOWN_TIMEOUT    int = 30
	READ_HEADER_TIMEOUT        int = 2
	WRITE_TIMEOUT              int = 5
	MAX_TEST_PATCHING_ATTEMPTS int = 10
	CONTAINER_TIMEOUT          int = 10     // in minutes
	CONTAINER_MEMORY_CAP       int = 2 * GB // in bytes
)

/*
	url := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	sha := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
*/

func loadTestingEnvVars() (err error) {
	testEnvFile, err := os.Open(GetPath("/orchestrator/internal/config/test-env-vars.txt"))
	if err != nil {
		return fmt.Errorf("Failed to open test env file: %w", err)
	}
	testEnvScanner := bufio.NewScanner(testEnvFile)
	for testEnvScanner.Scan() {
		line := testEnvScanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		TestingEnvSlice = append(TestingEnvSlice, line)
	}
	err = testEnvScanner.Err()
	if err != nil {
		return fmt.Errorf("Scanner failed: %w", err)
	}
	return nil
}

// Loads the env variables
func loadEnv() (err error) {
	err = godotenv.Load()
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
func Init() (err error) {
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
	WsDir = GetPath("/orchestrator/workspaces")

	err = loadEnv()
	if err != nil {
		return fmt.Errorf("Failed to load env variables: %w", err)
	}

	err = loadTestingEnvVars()
	if err != nil {
		return fmt.Errorf("Failed to load test env variables: %w", err)
	}

	return nil
}

// GetPath returns the relative path from the root orchestrator (RootDir + relPath)
func GetPath(relPath string) string {
	return filepath.Join(RootDir, relPath)
}
