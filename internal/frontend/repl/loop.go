package repl

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	primaryPrompt = "sysml> "
	contPrompt    = "  ...> "
)

// LineReader yields input lines; the prompt argument switches between primary
// and continuation prompts. Implemented by a readline adapter in cmd, and by a
// slice in tests. ReadLine returns io.EOF at end of input.
type LineReader interface {
	ReadLine(prompt string) (string, error)
}

// Loop runs the read/eval/print cycle until the reader returns io.EOF (Ctrl-D).
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
		if err != nil {
			return err
		}
		// Meta commands only at the primary prompt with an empty buffer.
		if buf.Len() == 0 && isMeta(line) {
			metaOut, quit, merr := s.RunMeta(line)
			printLines(w, metaOut)
			_ = w.Flush()
			if merr != nil {
				printLines(w, []string{"error: " + merr.Error()})
				_ = w.Flush()
			}
			if quit {
				return nil
			}
			continue
		}
		// Blank line at continuation prompt force-flushes.
		if buf.Len() > 0 && strings.TrimSpace(line) == "" {
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
		if needsContinuation(buf.String()) {
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
	if expr, ok := bareExpression(src); ok {
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

// bareExpression reports whether src reads as an expression and not as a
// namespace member. Only input the file grammar rejects is offered to the
// expression grammar, so every declaration keeps its meaning.
func bareExpression(src string) (string, bool) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", false
	}
	fp := parser.New(source.New("<repl-input>", []byte(src)))
	fp.ParseFile()
	if len(fp.Diagnostics) == 0 {
		return "", false
	}
	p := parser.New(source.New("<repl-input>", []byte(trimmed)))
	if expr := p.ParseExpression(); expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(trimmed) {
		return "", false
	}
	return trimmed, true
}

func printLines(w io.Writer, lines []string) {
	for _, l := range lines {
		_, _ = io.WriteString(w, l+"\n")
	}
}
