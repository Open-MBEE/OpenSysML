//go:build unix

package subprocess

import (
	"os"
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

// interrupts are the signals a terminal or a supervisor stops this process with.
var interrupts = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}
