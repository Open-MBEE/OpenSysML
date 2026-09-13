//go:build windows

package analysis

import "os/exec"

// ownProcessGroup is a no-op: Windows has no process group to end the children by.
func ownProcessGroup(*exec.Cmd) {}

// killProcessGroup ends the engine process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
