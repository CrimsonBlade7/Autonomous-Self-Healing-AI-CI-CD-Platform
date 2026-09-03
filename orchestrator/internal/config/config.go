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

var (
	WsDir string

	RootDir                     string
	Port                        string = "8080"
	GithubToken                 string
	RepositoryUrl               string
	GithubSecret                string
	AIEngineSecret              string
	AIEnginePort                string = "8000"
	TestingEnvSlice             []string
	AiEngineRequestTimeout      int = 5   // seconds
	ServerShutdownTimeout       int = 30  // seconds
	ReadHeaderTimeout           int = 2   // seconds
	WriteTimeout                int = 5   // seconds
	ContainerTimeout            int = 10  // minutes
	AIEngineRequestCloseTimeout int = 10  // seconds
	ContainerMemoryCap          int = 512 // MB
	MaxTestPatchingAttempts     int = 10
)

const (
	BYTE int = 1
	KB   int = 1e3 * BYTE
	MB   int = 1e3 * KB
	GB   int = 1e3 * MB
)

// Sets the root directory.
func resolveRootDir() error {
	// Use env variable for the project root if it exists.
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		RootDir = root
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get executable path: %w", err)
	}

	if exePath, err = filepath.EvalSymlinks(exePath); err != nil {
		return fmt.Errorf("Failed to resolve symlinks: %w", err)
	}

	// If the executable is in the current working directory, use the current working directory.
	// Otherwise, default to the folder containing the binary.
	if strings.Contains(exePath, os.TempDir()) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("Failed to get working directory: %w", err)
		}
		// go run from orchestrator/ should resolve to the monorepo root.
		if filepath.Base(cwd) == "orchestrator" {
			RootDir = filepath.Dir(cwd)
		} else {
			RootDir = cwd
		}
	} else {
		RootDir = filepath.Dir(exePath)
	}
	return nil
}

// Loads the environment variables to be injected into the docker container for testing.
func loadTestingEnvVars() error {
	// Target the nested config directory
	envPath := RelToAbsPath("orchestrator", "config", "test-env-vars.txt")

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
	envPath := RelToAbsPath("orchestrator", ".env")
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

// Returns an error if critical environment variables are missing.
func validateConfig() error {
	var missing []string

	if GithubToken == "" {
		missing = append(missing, "GITHUB_TOKEN")
	}
	if RepositoryUrl == "" {
		missing = append(missing, "GITHUB_REPOSITORY_URL")
	}
	if GithubSecret == "" {
		missing = append(missing, "GITHUB_WEBHOOK_SECRET")
	}
	if AIEngineSecret == "" {
		missing = append(missing, "AI_ENGINE_SECRET")
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
