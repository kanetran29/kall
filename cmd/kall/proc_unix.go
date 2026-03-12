//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setupProcessGroup puts the command in its own process group
// so we can kill the entire tree (sh + children) at once.
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcess kills the entire process group with SIGKILL.
// SIGKILL is used instead of SIGTERM because sh can trap SIGTERM and exit 0.
func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
