package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func setupWorkspace(t *testing.T) (string, *Config) {
	t.Helper()
	dir := t.TempDir()

	// Create two project directories
	os.MkdirAll(filepath.Join(dir, "alpha"), 0755)
	os.MkdirAll(filepath.Join(dir, "beta"), 0755)

	cfg := &Config{
		GlobalAliases: make(map[string]string),
		Projects: []Project{
			{Name: "alpha", Env: make(map[string]string), Aliases: make(map[string]string)},
			{Name: "beta", Env: make(map[string]string), Aliases: make(map[string]string)},
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

func TestRunLive(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	lives, doneCh := RunLive(dir, cfg, echoCmd())

	// Wait for all to finish
	for i := 0; i < len(cfg.Projects); i++ {
		<-doneCh
	}

	for _, lp := range lives {
		if !lp.IsDone() {
			t.Errorf("project %s: expected done", lp.Project)
		}
		if lp.ExitCode != 0 {
			t.Errorf("project %s: expected exit code 0, got %d", lp.Project, lp.ExitCode)
		}
		if !strings.Contains(lp.Output(), "hello") {
			t.Errorf("project %s: expected output to contain 'hello', got %q", lp.Project, lp.Output())
		}
	}
}

func TestRunLiveStreamsOutput(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	// Use a command that produces output
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"cmd", "/c", "echo streaming"}
	} else {
		args = []string{"echo", "streaming"}
	}

	lives, doneCh := RunLive(dir, cfg, args)

	// Wait for completion
	for i := 0; i < len(cfg.Projects); i++ {
		<-doneCh
	}

	// Verify output is accessible via Output()
	for _, lp := range lives {
		out := lp.Output()
		if !strings.Contains(out, "streaming") {
			t.Errorf("project %s: expected 'streaming' in output, got %q", lp.Project, out)
		}
	}
}

func TestRunLiveKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("kill test uses sleep, skipping on Windows")
	}
	dir, cfg := setupWorkspace(t)

	lives, doneCh := RunLive(dir, cfg, []string{"sleep", "30"})

	// Let the process start
	time.Sleep(100 * time.Millisecond)

	// Kill the first project
	lives[0].Kill()
	<-doneCh // wait for it to finish

	if !lives[0].IsDone() {
		t.Error("expected killed project to be done")
	}
	// The process was killed — verify it didn't run to completion
	if !strings.Contains(lives[0].Output(), "Killed") && lives[0].ExitCode == 0 {
		t.Error("expected killed indicator or non-zero exit code after kill")
	}

	// Kill the second to clean up
	lives[1].Kill()
	<-doneCh
}

func TestRunLiveRerun(t *testing.T) {
	dir, cfg := setupWorkspace(t)

	lives, doneCh := RunLive(dir, cfg, echoCmd())

	// Wait for initial run
	for i := 0; i < len(cfg.Projects); i++ {
		<-doneCh
	}

	if !strings.Contains(lives[0].Output(), "hello") {
		t.Fatalf("expected 'hello' in initial output, got %q", lives[0].Output())
	}

	// Rerun the first project
	lives[0].launch(doneCh, 0)
	<-doneCh

	if !lives[0].IsDone() {
		t.Error("expected rerun project to be done")
	}
	if !strings.Contains(lives[0].Output(), "hello") {
		t.Errorf("expected 'hello' in rerun output, got %q", lives[0].Output())
	}
}
