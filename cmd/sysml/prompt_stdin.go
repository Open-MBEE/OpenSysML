package main

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// plainReader yields the lines it is read from, byte for byte: nothing is a
// keystroke, so a TAB is indentation, string content or comment text. Each
// prompt is written to out before it waits, when out is given.
type plainReader struct {
	in        *bufio.Reader
	out       io.Writer
	interrupt string // a line that reads as repl.ErrInterrupt, when nonempty
}

// ReadLine writes the prompt and reads the next line, without its line ending, and
// reports io.EOF once input ends: a last line sent without a newline is still a
// line, and the read after it is the end.
func (r *plainReader) ReadLine(prompt string) (string, error) {
	if r.out != nil {
		if _, err := io.WriteString(r.out, prompt); err != nil {
			return "", err
		}
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
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if r.interrupt != "" && line == r.interrupt {
		return "", repl.ErrInterrupt
	}
	return line, nil
}
