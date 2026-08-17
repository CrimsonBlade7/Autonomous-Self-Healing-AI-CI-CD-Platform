package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var RootDir string
var Port string = "8080"
var WsDir string
var GithubToken string
var RepositoryUrl string
var GithubSecret string
var AIEngineSecret string
var AIEnginePort string = "8000"
var TestingEnvSlice []string

var AiEngineRequestTimeout int = 5
var ServerShutdownTimeout int = 30
var ReadHeaderTimeout int = 2
var WriteTimeout int = 5
var MaxTestPatchingAttempts int = 10
var ContainerTimeout int = 10       // in minutes
var ContainerMemoryCap int = 2 * GB // in bytes

const (
	BYTE int = 1
	KB   int = 1e3 * BYTE
	MB   int = 1e3 * KB
	GB   int = 1e3 * MB
)

// Sets the root directory.
func resolveRootDir() error {
	if envRoot := os.Getenv("PROJECT_ROOT"); envRoot != "" {
		RootDir = envRoot
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get executable path: %w", err)
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("Failed to resolve symlinks: %w", err)
	}

	// Under `go run` (temp folder), use current working directory.
	// Otherwise, default to the folder containing the binary.
	if strings.Contains(exePath, os.TempDir()) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("Failed to get working directory: %w", err)
		}
		RootDir = cwd
	} else {
		RootDir = filepath.Dir(exePath)
	}
	return nil
}

// Loads the environment variables to be injected into the docker container for testing.
func loadTestingEnvVars() error {
	// Target the nested config directory
	envPath := RelToAbsPath("config", "test-env-vars.txt")

	testEnvFile, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("Failed to open test env file at %s: %w", envPath, err)
	}
	defer testEnvFile.Close()

	testEnvScanner := bufio.NewScanner(testEnvFile)
	for testEnvScanner.Scan() {
		line := strings.TrimSpace(testEnvScanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		TestingEnvSlice = append(TestingEnvSlice, line)
	}

	if err := testEnvScanner.Err(); err != nil {
		return fmt.Errorf("scanner Failed: %w", err)
	}
	return nil
}

// Loads the environment variables.
func loadEnv() error {
	// Root level .env file
	envPath := RelToAbsPath(".env")
	if err := godotenv.Load(envPath); err != nil {
		// Non-fatal if running in environments where variables are injected directly (e.g., Docker/K8s)
		if !os.IsNotExist(err) {
			return fmt.Errorf("Failed to load .env file from %s: %w", envPath, err)
		}
	}

	// Environment variable assignments with fallback defaults
	GithubToken = os.Getenv("GITHUB_TOKEN")
	RepositoryUrl = os.Getenv("GITHUB_REPOSITORY_URL")
	GithubSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	AIEngineSecret = os.Getenv("AI_ENGINE_SECRET")

	if p := os.Getenv("PORT"); p != "" {
		Port = p
	}

	if aiPort := os.Getenv("AI_ENGINE_PORT"); aiPort != "" {
		AIEnginePort = aiPort
	}

	// Optional numeric overrides from environment

	if valAiTimeout := os.Getenv("AI_ENGINE_REQUEST_TIMEOUT"); valAiTimeout != "" {
		parsedVal, err := strconv.Atoi(valAiTimeout)
		if err == nil {
			AiEngineRequestTimeout = parsedVal
		}
	}

	if valServerShutdown := os.Getenv("SERVER_SHUTDOWN_TIMEOUT"); valServerShutdown != "" {
		parsedVal, err := strconv.Atoi(valServerShutdown)
		if err == nil {
			ServerShutdownTimeout = parsedVal
		}
	}

	if valReadHeader := os.Getenv("READ_HEADER_TIMEOUT"); valReadHeader != "" {
		parsedVal, err := strconv.Atoi(valReadHeader)
		if err == nil {
			ReadHeaderTimeout = parsedVal
		}
	}

	if valWriteTimeout := os.Getenv("WRITE_TIMEOUT"); valWriteTimeout != "" {
		parsedVal, err := strconv.Atoi(valWriteTimeout)
		if err == nil {
			WriteTimeout = parsedVal
		}
	}

	if valMaxTestAttempts := os.Getenv("MAX_TEST_PATCHING_ATTEMPTS"); valMaxTestAttempts != "" {
		parsedVal, err := strconv.Atoi(valMaxTestAttempts)
		if err == nil {
			MaxTestPatchingAttempts = parsedVal
		}
	}

	if valContainerTimeout := os.Getenv("CONTAINER_TIMEOUT"); valContainerTimeout != "" {
		parsedVal, err := strconv.Atoi(valContainerTimeout)
		if err == nil {
			ContainerTimeout = parsedVal
		}
	}

	if valContainerMemoryCap := os.Getenv("CONTAINER_MEMORY_CAP"); valContainerMemoryCap != "" {
		parsedVal, err := strconv.Atoi(valContainerMemoryCap)
		if err == nil {
			ContainerMemoryCap = parsedVal
		}
	}

	return nil
}

// Returns an error if certain environment variables are missing.
func validateConfig() error {
	var missing []string

	if GithubToken == "" {
		missing = append(missing, "GITHUB_TOKEN")
	}
	if RepositoryUrl == "" {
		missing = append(missing, "GITHUB_REPOSITORY_URL")
	}

	if len(missing) > 0 {
		return fmt.Errorf("Missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// Initializes the global variables.
func Init() error {
	if err := resolveRootDir(); err != nil {
		return fmt.Errorf("Failed to resolve root directory: %w", err)
	}

	if err := loadEnv(); err != nil {
		return fmt.Errorf("Failed to load env variables: %w", err)
	}

	if err := validateConfig(); err != nil {
		return fmt.Errorf("Invalid configuration: %w", err)
	}

	WsDir = RelToAbsPath("workspaces")

	if err := loadTestingEnvVars(); err != nil {
		return fmt.Errorf("Failed to load test env variables: %w", err)
	}

	return nil
}

// Joins and prefixes the root to create the absolute path.
func RelToAbsPath(relPath ...string) string {
	return filepath.Join(append([]string{RootDir}, relPath...)...)
}
