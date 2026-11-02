package repl

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/notebook"
)

const (
	primaryPrompt = "sysml> "
	contPrompt    = "  ...> "
)

// ErrInterrupt is returned by a LineReader, with the text typed so far, when the
// line is interrupted (Ctrl-C).
var ErrInterrupt = errors.New("interrupt")

// LineReader yields input lines; the prompt argument switches between primary
// and continuation prompts. Implemented by a readline adapter in cmd, and by a
// slice in tests. ReadLine returns io.EOF at end of input and ErrInterrupt when
// the line is interrupted.
type LineReader interface {
	ReadLine(prompt string) (string, error)
}

// Loop runs until io.EOF (Ctrl-D) or an interrupt at an empty prompt (Ctrl-C);
// any other interrupt discards the line and the buffered continuation.
func Loop(r LineReader, out io.Writer, s *Session) error {
	w := bufio.NewWriter(out)
	defer func() { _ = w.Flush() }()
	var buf strings.Builder
	prompt := primaryPrompt
	for {
		line, err := r.ReadLine(prompt)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, ErrInterrupt) {
			if buf.Len() == 0 && line == "" {
				return nil
			}
			buf.Reset()
			prompt = primaryPrompt
			continue
		}
		if err != nil {
			return err
		}
		// Meta commands only at the primary prompt with an empty buffer.
		if buf.Len() == 0 && notebook.IsCommand(line) {
			metaOut, quit, merr := s.RunMeta(line)
			printLines(w, metaOut)
			_ = w.Flush()
			if merr != nil {
				printLines(w, metaErrorLines(merr))
				_ = w.Flush()
			}
			if quit {
				return nil
			}
			continue
		}
		// Blank line at continuation prompt force-flushes, unless the buffer is
		// inside an open literal or comment, where the blank line is content.
		if buf.Len() > 0 && strings.TrimSpace(line) == "" && !openLiteral(buf.String()) {
			submit(w, s, buf.String())
			buf.Reset()
			prompt = primaryPrompt
			_ = w.Flush()
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
		if notebook.NeedsContinuation(buf.String()) {
			prompt = contPrompt
			continue
		}
		if strings.TrimSpace(buf.String()) != "" {
			submit(w, s, buf.String())
		}
		buf.Reset()
		prompt = primaryPrompt
		_ = w.Flush()
	}
}

// submit evaluates a bare expression as %eval does, so `2 + 3` at the prompt
// answers rather than failing as a declaration; anything else joins the buffer.
func submit(w io.Writer, s *Session, src string) {
	if expr, ok := notebook.BareExpression(src); ok {
		lines, err := s.EvalBare(expr)
		if err != nil {
			printLines(w, []string{"error: " + err.Error()})
			return
		}
		printLines(w, lines)
		return
	}
	printLines(w, renderResult(s.Submit(src), s.Verbosity()))
}

// metaErrorLines prints a meta command's error: every line of a usage error,
// the first under the error prefix as the kernel's value; other errors as before.
func metaErrorLines(err error) []string {
	var usage *UsageError
	if errors.As(err, &usage) && len(usage.Lines) > 0 {
		return append([]string{errPrefix + usage.Error()}, usage.Lines[1:]...)
	}
	return []string{errPrefix + err.Error()}
}

func printLines(w io.Writer, lines []string) {
	for _, l := range lines {
		_, _ = io.WriteString(w, l+"\n")
	}
}
