//go:build !unix

package main

import (
	"os"
	"os/exec"
)

func interruptSignals() []os.Signal { return []os.Signal{os.Interrupt} }

func setProcessGroup(cmd *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
