package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Result holds the output and exit code from running a command in one project.
type Result struct {
	Project  string
	Command  string
	Output   string
	ExitCode int
}

// LiveProject tracks a running command with live-streaming output.
// Supports kill and rerun. A generation counter prevents stale goroutines
// from corrupting state after a restart.
type LiveProject struct {
	Project  string
	Command  string
	Dir      string
	Shell    string
	ExtraEnv []string // additional env vars from config

	mu       sync.Mutex
	buf      bytes.Buffer
	Done     bool
	ExitCode int
	cmd      *exec.Cmd
	killed   bool
	gen      int
}

// Write implements io.Writer so exec.Cmd can stream stdout/stderr into it.
func (lp *LiveProject) Write(p []byte) (int, error) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return lp.buf.Write(p)
}

// Output returns all output captured so far (safe to call while running).
func (lp *LiveProject) Output() string {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return lp.buf.String()
}

// IsDone returns whether the command has finished.
func (lp *LiveProject) IsDone() bool {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return lp.Done
}

// Kill sends SIGKILL to the running process group. No-op if already done.
func (lp *LiveProject) Kill() {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	if lp.Done || lp.cmd == nil {
		return
	}
	lp.killed = true
	killProcess(lp.cmd)
}

// launch starts (or restarts) the command. If a previous command is still
// running, it is killed first. The generation counter ensures only the
// latest goroutine updates state. doneCh receives idx when done.
func (lp *LiveProject) launch(doneCh chan<- int, idx int) {
	lp.mu.Lock()

	// Kill existing process if still running
	if !lp.Done && lp.cmd != nil {
		lp.killed = true
		killProcess(lp.cmd)
	}

	// Reset state for new run
	lp.gen++
	myGen := lp.gen
	lp.buf.Reset()
	lp.Done = false
	lp.ExitCode = 0
	lp.killed = false
	lp.cmd = nil
	lp.mu.Unlock()

	go func() {
		cmd := shellCommand(lp.Command, lp.Dir, lp.Shell)
		setupProcessGroup(cmd)
		cmd.Stdout = lp
		cmd.Stderr = lp
		cmd.Env = append(cleanEnv(), lp.ExtraEnv...)

		lp.mu.Lock()
		lp.cmd = cmd
		lp.mu.Unlock()

		err := cmd.Run()

		lp.mu.Lock()
		if lp.gen != myGen {
			lp.mu.Unlock()
			return // superseded by a restart
		}

		exitCode := 0
		if err != nil {
			if lp.killed {
				exitCode = 130
				lp.buf.WriteString("Killed.\n")
			} else if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
				lp.buf.WriteString(err.Error())
			}
		}
		lp.ExitCode = exitCode
		lp.Done = true
		lp.mu.Unlock()

		doneCh <- idx
	}()
}

// resolveCommand expands alias: checks project aliases first, then global.
func resolveCommand(proj Project, globalAliases map[string]string, args []string) []string {
	if alias, ok := proj.Aliases[args[0]]; ok {
		parts := strings.Fields(alias)
		return append(parts, args[1:]...)
	}
	if alias, ok := globalAliases[args[0]]; ok {
		parts := strings.Fields(alias)
		return append(parts, args[1:]...)
	}
	return args
}

// projectDir returns the working directory for a project, respecting dir override.
func projectDir(root string, proj Project) string {
	base := filepath.Join(root, proj.Name)
	if proj.Dir != "" {
		return filepath.Join(base, proj.Dir)
	}
	return base
}

// projectShell returns the shell to use: per-project > global settings > default.
func projectShell(proj Project, settings Settings) string {
	if proj.Shell != "" {
		return proj.Shell
	}
	if settings.Shell != "" {
		return settings.Shell
	}
	return ""
}

// projectEnv returns extra env vars for a project as KEY=VALUE strings.
func projectEnv(proj Project) []string {
	var env []string
	for k, v := range proj.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// shellCommand creates an exec.Cmd that runs cmdStr through a shell.
func shellCommand(cmdStr, dir, shell string) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else if shell != "" {
		cmd = exec.Command(shell, "-c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	cmd.Dir = dir
	return cmd
}

// cleanEnv returns the current environment with problematic variables removed.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		key := e[:strings.Index(e, "=")]
		switch key {
		case "CLAUDECODE":
			continue
		default:
			env = append(env, e)
		}
	}
	return env
}

// RunParallel executes args across all projects in parallel, returning results
// in the same order as config.Projects. Used for piped/non-TTY output.
func RunParallel(root string, cfg *Config, args []string) []Result {
	results := make([]Result, len(cfg.Projects))
	var wg sync.WaitGroup

	sem := makeSemaphore(cfg.Settings.Concurrency)

	for i, project := range cfg.Projects {
		wg.Add(1)
		go func(idx int, proj Project) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			cmdArgs := resolveCommand(proj, cfg.GlobalAliases, args)
			cmdStr := strings.Join(cmdArgs, " ")
			dir := projectDir(root, proj)
			shell := projectShell(proj, cfg.Settings)

			cmd := shellCommand(cmdStr, dir, shell)
			cmd.Env = append(cleanEnv(), projectEnv(proj)...)
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = 1
					buf.WriteString(err.Error())
				}
			}

			results[idx] = Result{
				Project:  proj.DisplayName(),
				Command:  cmdStr,
				Output:   buf.String(),
				ExitCode: exitCode,
			}
		}(i, project)
	}

	wg.Wait()
	return results
}

// RunLive launches all commands in parallel and returns LiveProject handles
// that stream output in real-time. doneCh receives the project index each
// time a command finishes (including after restarts).
func RunLive(root string, cfg *Config, args []string) ([]*LiveProject, chan int) {
	lives := make([]*LiveProject, len(cfg.Projects))
	doneCh := make(chan int, len(cfg.Projects)*4)

	for i, proj := range cfg.Projects {
		cmdArgs := resolveCommand(proj, cfg.GlobalAliases, args)
		cmdStr := strings.Join(cmdArgs, " ")

		lives[i] = &LiveProject{
			Project:  proj.DisplayName(),
			Command:  cmdStr,
			Dir:      projectDir(root, proj),
			Shell:    projectShell(proj, cfg.Settings),
			ExtraEnv: projectEnv(proj),
		}
		lives[i].launch(doneCh, i)
	}

	return lives, doneCh
}

// makeSemaphore creates a buffered channel to limit concurrency, or nil if unlimited.
func makeSemaphore(n int) chan struct{} {
	if n <= 0 {
		return nil
	}
	return make(chan struct{}, n)
}
