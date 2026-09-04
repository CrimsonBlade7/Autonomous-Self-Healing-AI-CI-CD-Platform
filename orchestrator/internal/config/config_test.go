package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelToAbsPath(t *testing.T) {
	prev := OrchRootDir
	t.Cleanup(func() { OrchRootDir = prev })

	OrchRootDir = filepath.FromSlash("/repo")
	got := RelToAbsPath("orchestrator", ".env")
	want := filepath.Join("/repo", "orchestrator", ".env")
	if got != want {
		t.Errorf("RelToAbsPath = %q, want %q", got, want)
	}
}

func TestValidateConfig_MissingRequired(t *testing.T) {
	prevToken, prevURL, prevGH, prevAI := GithubToken, RepositoryUrl, GithubSecret, AIEngineSecret
	t.Cleanup(func() {
		GithubToken, RepositoryUrl, GithubSecret, AIEngineSecret = prevToken, prevURL, prevGH, prevAI
	})

	GithubToken = ""
	RepositoryUrl = ""
	GithubSecret = ""
	AIEngineSecret = ""

	err := validateConfig()
	if err == nil {
		t.Fatal("expected missing env error")
	}
	msg := err.Error()
	for _, name := range []string{"GITHUB_TOKEN", "GITHUB_REPOSITORY_URL", "GITHUB_WEBHOOK_SECRET", "AI_ENGINE_SECRET"} {
		if !strings.Contains(msg, name) {
			t.Errorf("error %q missing %s", msg, name)
		}
	}
}

func TestValidateConfig_OK(t *testing.T) {
	prevToken, prevURL, prevGH, prevAI := GithubToken, RepositoryUrl, GithubSecret, AIEngineSecret
	t.Cleanup(func() {
		GithubToken, RepositoryUrl, GithubSecret, AIEngineSecret = prevToken, prevURL, prevGH, prevAI
	})

	GithubToken = "t"
	RepositoryUrl = "https://example.com/repo.git"
	GithubSecret = "s"
	AIEngineSecret = "a"

	if err := validateConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRootDir_UsesProjectRootEnv(t *testing.T) {
	prev := OrchRootDir
	t.Cleanup(func() {
		OrchRootDir = prev
		_ = os.Unsetenv("PROJECT_ROOT")
	})

	want := filepath.Join(t.TempDir(), "root")
	t.Setenv("PROJECT_ROOT", want)

	if err := resolveRootDir(); err != nil {
		t.Fatalf("resolveRootDir: %v", err)
	}
	if OrchRootDir != want {
		t.Errorf("RootDir = %q, want %q", OrchRootDir, want)
	}
}

func TestLoadTestingEnvVars(t *testing.T) {
	prevRoot, prevSlice := OrchRootDir, TestingEnvSlice
	t.Cleanup(func() {
		OrchRootDir = prevRoot
		TestingEnvSlice = prevSlice
	})

	root := t.TempDir()
	cfgDir := filepath.Join(root, "orchestrator", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "# comment\n\nFOO=bar\n  BAZ=qux  \n"
	if err := os.WriteFile(filepath.Join(cfgDir, "test-env-vars.txt"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	OrchRootDir = root
	TestingEnvSlice = nil
	if err := loadTestingEnvVars(); err != nil {
		t.Fatalf("loadTestingEnvVars: %v", err)
	}
	if len(TestingEnvSlice) != 2 || TestingEnvSlice[0] != "FOO=bar" || TestingEnvSlice[1] != "BAZ=qux" {
		t.Errorf("TestingEnvSlice = %#v", TestingEnvSlice)
	}
}