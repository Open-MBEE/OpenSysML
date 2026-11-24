//go:build !windows

package subprocess

import (
	"os/exec"
	"syscall"
)

// Group starts cmd in a process group of its own and ends the whole group when cmd's
// context ends, so a launcher script's children do not outlive it.
func Group(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
