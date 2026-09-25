package project

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/chzyer/readline"
)

// stdin holds the model read from standard input: the stream can be read only
// once, so one read answers every later "-".
var stdin struct {
	once sync.Once
	data []byte
	err  error
}

// ReadFile returns the contents of the model at path together with the name it
// is reported under, reading standard input when path names it.
func ReadFile(path string) (string, []byte, error) {
	if IsStdin(path) {
		data, err := readStdin()
		return StdinName, data, err
	}
	// #nosec G304 -- the file is one the user named, or one found under a
	// directory or pattern they named.
	data, err := os.ReadFile(path)
	return path, data, err
}

// readStdin reads the whole of standard input, refusing a terminal rather than
// waiting on it so that a "-" with nothing redirected is reported.
func readStdin() ([]byte, error) {
	stdin.once.Do(func() {
		if isTerminal(os.Stdin) {
			stdin.err = errors.New("standard input is a terminal; redirect it or name a file")
			return
		}
		stdin.data, stdin.err = io.ReadAll(os.Stdin)
	})
	return stdin.data, stdin.err
}

// isTerminal reports whether f is a terminal. It asks the device itself rather
// than reading a mode bit, which /dev/null and every other character device set.
func isTerminal(f *os.File) bool {
	return readline.IsTerminal(int(f.Fd()))
}
