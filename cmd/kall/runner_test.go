package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupWorkspace(t *testing.T) (string, *Config) {
	t.Helper()
	dir := t.TempDir()

	// Create two project directories
	os.MkdirAll(filepath.Join(dir, "alpha"), 0755)
	os.MkdirAll(filepath.Join(dir, "beta"), 0755)

	cfg := &Config{
		Projects: []Project{
			{Name: "alpha", Aliases: make(map[string]string)},
			{Name: "beta", Aliases: make(map[string]string)},
		},
	}

	return dir, cfg
}

func echoCmd() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "echo hello"}
	}
	return []string{"echo", "hello"}
}

func failCmd() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "exit /b 1"}
	}
	return []string{"false"}
}

func TestRunParallel(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	results := RunParallel(dir, cfg, echoCmd())

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.ExitCode != 0 {
			t.Errorf("project %s: expected exit code 0, got %d (output: %s)", r.Project, r.ExitCode, r.Output)
		}
		if !strings.Contains(r.Output, "hello") {
			t.Errorf("project %s: expected output to contain 'hello', got %q", r.Project, r.Output)
		}
	}
}

func TestRunParallelFailure(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	results := RunParallel(dir, cfg, failCmd())

	for _, r := range results {
		if r.ExitCode == 0 {
			t.Errorf("project %s: expected non-zero exit code", r.Project)
		}
	}
}

func TestRunParallelWithAlias(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	if runtime.GOOS == "windows" {
		cfg.Projects[0].Aliases["greet"] = "cmd /c echo aliased"
	} else {
		cfg.Projects[0].Aliases["greet"] = "echo aliased"
	}

	results := RunParallel(dir, cfg, []string{"greet"})

	// alpha should use the alias
	if !strings.Contains(results[0].Output, "aliased") {
		t.Errorf("alpha: expected alias output, got %q", results[0].Output)
	}

	// beta has no alias for "greet" — will fail (command not found)
	if results[1].ExitCode == 0 {
		t.Errorf("beta: expected failure for unknown command 'greet'")
	}
}

func TestRunParallelPreservesOrder(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	results := RunParallel(dir, cfg, echoCmd())

	if results[0].Project != "alpha" {
		t.Errorf("expected first result to be 'alpha', got '%s'", results[0].Project)
	}
	if results[1].Project != "beta" {
		t.Errorf("expected second result to be 'beta', got '%s'", results[1].Project)
	}
}
