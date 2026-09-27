//go:build wasm

package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// newLineInput opens the prompt's line reader over standard input. A WebAssembly
// host has no line editor to hand: lines arrive one at a time from whatever the
// host wired up, without history, completion or an interrupt to catch. The session
// is passed for the signature it has on every build and is not consulted. The
// returned function closes the reader, which is nothing to do over standard input.
func newLineInput(_ *repl.Session) (repl.LineReader, func() error, error) {
	return &plainReader{in: bufio.NewReader(os.Stdin), out: os.Stdout}, func() error { return nil }, nil
}

// plainReader yields the lines it is read from, writing each prompt before it waits.
type plainReader struct {
	in  *bufio.Reader
	out io.Writer
}

// ReadLine writes the prompt and reads the next line, without its line ending, and
// reports io.EOF once input ends: a last line the host sent without a newline is
// still a line, and the read after it is the end.
func (r *plainReader) ReadLine(prompt string) (string, error) {
	if _, err := io.WriteString(r.out, prompt); err != nil {
		return "", err
	}
	line, err := r.in.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return "", err
		}
		if line == "" {
			return "", io.EOF
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

// isTerminal reports whether the file descriptor is a terminal: no WebAssembly host
// offers a terminal query (WASI preview 1 has no isatty, a browser has no device),
// so input is never reported as one and a "-" that named it reads the lines the
// host sends, ending at end of input, exactly as a redirected pipe does natively.
func isTerminal(int) bool { return false }

// terminalWidthOf is the width in cells of the terminal on the file descriptor: 0,
// for the same reason isTerminal reports none, so renderings are written unbounded.
func terminalWidthOf(int) int { return 0 }
