//go:build windows

package main

import "os/exec"

func setupProcessGroup(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
