//go:build !unix

package analysis

import "os/exec"

// ownProcessGroup is a no-op: only the unix targets have process groups; Windows, Plan 9
// and the WebAssembly hosts have no syscall.SysProcAttr.Setpgid.
func ownProcessGroup(*exec.Cmd) {
	// Intentionally empty: there is no process group to mark.
}

// killProcessGroup ends the engine process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
