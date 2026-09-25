//go:build !wasm

package main

import (
	"io"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// rlReader answers repl.LineReader over readline's line editor.
type rlReader struct{ rl *readline.Instance }

func (r *rlReader) ReadLine(prompt string) (string, error) {
	r.rl.SetPrompt(prompt)
	line, err := r.rl.Readline()
	if err == readline.ErrInterrupt { // Ctrl-C clears line (continue REPL)
		return "", nil
	}
	if err == io.EOF { // Ctrl-D exits REPL
		return "", io.EOF
	}
	return line, err
}

// newLineInput opens the prompt's line reader: readline, with the session's history
// file and completion, an interrupted line clearing rather than ending the session,
// and end of input closing it. The returned function closes the reader.
func newLineInput(sess *repl.Session) (repl.LineReader, func() error, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sysml> ",
		HistoryFile:     historyPath(),
		AutoComplete:    &sessionCompleter{sess: sess},
		InterruptPrompt: "^C",
		EOFPrompt:       "bye",
	})
	if err != nil {
		return nil, nil, err
	}
	return &rlReader{rl: rl}, rl.Close, nil
}

// isTerminal reports whether the file descriptor is a terminal. It asks the device
// itself rather than reading a mode bit, which /dev/null and every other character
// device set.
func isTerminal(fd int) bool { return readline.IsTerminal(fd) }

// terminalWidthOf is the width in cells of the terminal on the file descriptor, 0
// when it is not a terminal or will not report one.
func terminalWidthOf(fd int) int {
	width, _, err := readline.GetSize(fd)
	if err != nil {
		return 0
	}
	return width
}
