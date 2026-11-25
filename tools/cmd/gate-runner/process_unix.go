//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func interruptSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}
