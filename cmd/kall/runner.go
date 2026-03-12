package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
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

// RunParallel executes args across all projects in parallel, returning results
// in the same order as config.Projects.
func RunParallel(root string, cfg *Config, args []string) []Result {
	results := make([]Result, len(cfg.Projects))
	var wg sync.WaitGroup

	for i, project := range cfg.Projects {
		wg.Add(1)
		go func(idx int, proj Project) {
			defer wg.Done()

			// Resolve alias: if the first arg matches an alias, expand it
			var cmdArgs []string
			if alias, ok := proj.Aliases[args[0]]; ok {
				parts := strings.Fields(alias)
				cmdArgs = append(parts, args[1:]...)
			} else {
				cmdArgs = args
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Dir = filepath.Join(root, proj.Name)

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
				Project:  proj.Name,
				Command:  strings.Join(cmdArgs, " "),
				Output:   buf.String(),
				ExitCode: exitCode,
			}
		}(i, project)
	}

	wg.Wait()
	return results
}
