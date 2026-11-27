//go:build !unix

package subprocess

import (
	"os"
	"os/exec"
)

// Group is a no-op: off Unix there is no process group to end, so cmd's context ends cmd alone.
func Group(*exec.Cmd) {
	// Intentionally empty: there is no process group to mark.
}

// interrupts are the signals a console or terminal stops this process with.
var interrupts = []os.Signal{os.Interrupt}
