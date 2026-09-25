package main

import (
	"fmt"
	"os"

	"github.com/chzyer/readline"
)

// Exit statuses of any run: a verdict the model decided false is 1, anything
// that stopped the run from being carried out at all is 2, and a run carried
// out in part, writing what it could and naming what it could not, is 3.
const (
	exitHolds       = 0
	exitFailed      = 1
	exitUnevaluable = 2
	exitPartial     = 3
)

// fail reports on stderr what stopped the run, returning the status of a run
// that decided nothing.
func fail(err error) int {
	fmt.Fprintln(os.Stderr, commandPrefix+err.Error())
	return exitUnevaluable
}

// atTerminal reports whether lines are being read from a terminal, which is what
// makes an unusable model a session to fix it in rather than a failed run.
func atTerminal() bool {
	return readline.IsTerminal(int(os.Stdin.Fd()))
}
