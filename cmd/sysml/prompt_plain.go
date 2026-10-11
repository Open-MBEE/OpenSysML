//go:build wasm

package main

import (
	"bufio"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// newLineInput opens the prompt's line reader over standard input. A WebAssembly
// host has no line editor to hand: lines arrive one at a time from whatever the
// host wired up. A browser page gets completion through the session and may send
// hostInterruptLine for Ctrl-C. The returned function closes the reader, which is
// nothing to do over standard input.
func newLineInput(sess *repl.Session) (repl.LineReader, func() error, error) {
	exposeCompletion(sess)
	return &plainReader{in: bufio.NewReader(os.Stdin), out: os.Stdout, interrupt: hostInterruptLine}, func() error { return nil }, nil
}

// isTerminal reports whether the file descriptor is a terminal: no WebAssembly host
// offers a terminal query (WASI preview 1 has no isatty, a browser has no device),
// so input is never reported as one and a "-" that named it reads the lines the
// host sends, ending at end of input, exactly as a redirected pipe does natively.
func isTerminal(int) bool { return false }

// terminalWidthOf is the width in cells of the terminal on the file descriptor: 0,
// for the same reason isTerminal reports none, so renderings are written unbounded.
func terminalWidthOf(int) int { return 0 }
