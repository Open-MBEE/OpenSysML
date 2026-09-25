//go:build windows || wasm

package analysis

import "os/exec"

// ownProcessGroup is a no-op: Windows, a browser and a WebAssembly host all lack the
// process-group control the unix build sets (GOARCH=wasm covers js/wasm and wasip1, where
// syscall.SysProcAttr has no Setpgid and there is no process to group at all).
func ownProcessGroup(*exec.Cmd) {
	// Intentionally empty: there is no process group to mark.
}

// killProcessGroup ends the engine process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
