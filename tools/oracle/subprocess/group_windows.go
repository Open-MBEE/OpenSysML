//go:build windows

package subprocess

import "os/exec"

// Group is a no-op: Windows has no process group to end, so cmd's context ends cmd alone.
func Group(*exec.Cmd) {
	// Intentionally empty: there is no process group to mark.
}
