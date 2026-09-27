//go:build !wasm

package project

import (
	"os"

	"github.com/chzyer/readline"
)

// isTerminal reports whether f is a terminal. It asks the device itself rather
// than reading a mode bit, which /dev/null and every other character device set.
func isTerminal(f *os.File) bool {
	return readline.IsTerminal(int(f.Fd()))
}
