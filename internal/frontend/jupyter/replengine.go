package jupyter

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// errPrefix marks a line the prompt prints for a failure.
const errPrefix = "error: "

// REPLEngine runs cells against a REPL session: each cell is split into the
// statements the prompt would read from it, and each statement is submitted as
// the prompt submits it, so a notebook and a terminal session behave alike.
type REPLEngine struct {
	session *repl.Session
}

// NewREPLEngine is an engine over the session.
func NewREPLEngine(session *repl.Session) *REPLEngine {
	return &REPLEngine{session: session}
}

// Session is the session the engine runs.
func (e *REPLEngine) Session() *repl.Session { return e.session }

// Execute runs the cell's statements in order and stops at the first that
// fails, as a cell of any other kernel does.
func (e *REPLEngine) Execute(code string, out Output) error {
	for _, stmt := range repl.Statements(code) {
		if err := e.execute(stmt, out); err != nil {
			return err
		}
	}
	return nil
}

// execute runs one statement.
func (e *REPLEngine) execute(stmt repl.Statement, out Output) error {
	if stmt.Meta {
		return e.meta(stmt.Text, out)
	}
	if expr, ok := repl.BareExpression(stmt.Text); ok {
		lines, err := e.session.EvalBare(expr)
		if err != nil {
			return execError("EvaluationError", err.Error(), lines)
		}
		out.Result(textBundle(lines))
		return nil
	}
	result := e.session.Submit(stmt.Text)
	lines := repl.RenderResult(result, e.session.Verbosity())
	if result.Failed(e.session.Verbosity()) {
		return execError("SubmissionError", summary(lines), lines)
	}
	out.Stream("stdout", joinLines(lines))
	return nil
}

// renderError is a %render or %render-document that did not run: a usage
// problem, shown as the prompt prints it, or a view or document that could not
// be rendered.
func renderError(err error) error {
	var usage *repl.UsageError
	if errors.As(err, &usage) {
		return execError("UsageError", usage.Error(), usage.Lines)
	}
	return execError("RenderError", err.Error(), []string{errPrefix + err.Error()})
}

// meta runs a `%` command. The ones whose output has a richer form than text
// are rendered through the session's typed surface; the rest print as the
// prompt prints them.
func (e *REPLEngine) meta(line string, out Output) error {
	cmd, args := splitMeta(line)
	if !repl.KnownMeta(cmd) {
		lines, _, _ := e.session.RunMeta(line)
		return execError("UnknownCommand", summary(lines), lines)
	}
	switch cmd {
	case "%render":
		rendered, err := e.session.Render(args)
		if err != nil {
			return renderError(err)
		}
		out.Display(formBundle(rendered.Form, rendered.Lines), nil)
		return nil
	case "%render-document":
		// A trailing `html` asks for the document as HTML rather than the
		// Markdown the prompt prints; the arguments before it are the prompt's.
		html := len(args) > 0 && args[len(args)-1] == "html"
		if html {
			args = args[:len(args)-1]
		}
		lines, rendered, err := e.session.RenderDocument(strings.Join(args, " "), html)
		if err != nil {
			return renderError(err)
		}
		switch {
		case !rendered:
			out.Stream("stdout", joinLines(lines))
		case html:
			out.Display(htmlBundle(lines), nil)
		default:
			out.Display(markdownBundle(lines), nil)
		}
		return nil
	}
	lines, quit, err := e.session.RunMeta(line)
	if err != nil {
		name := "CommandError"
		if errors.Is(err, runtime.ErrInterrupted) {
			name = "KeyboardInterrupt"
		}
		return execError(name, err.Error(), append(lines, errPrefix+err.Error()))
	}
	if quit {
		// %quit ends a terminal session; a notebook's kernel is ended by the
		// front end, so the command is noted rather than acted on.
		lines = append(lines, "use the notebook's shutdown to end the kernel")
	}
	if failed(lines) {
		name := "CommandError"
		if interrupted(lines) {
			name = "KeyboardInterrupt"
		}
		return execError(name, summary(lines), lines)
	}
	if cmd == "%features" && len(args) >= 2 && args[len(args)-1] == "json" {
		if bundle, ok := jsonBundle(lines); ok {
			out.Display(bundle, nil)
			return nil
		}
	}
	out.Stream("stdout", joinLines(lines))
	return nil
}

// splitMeta reads a meta command line as its command and arguments.
func splitMeta(line string) (string, []string) {
	fields := repl.MetaArgs(line)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// failed reports whether the lines a command printed report a failure: the
// prompt prefixes one with "error:".
func failed(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, errPrefix) {
			return true
		}
	}
	return false
}

// interrupted reports whether the lines report the run was interrupted.
func interrupted(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, runtime.ErrInterrupted.Error()) {
			return true
		}
	}
	return false
}

// summary is the first failure line without its prefix, or the first line.
func summary(lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, errPrefix) {
			return strings.TrimPrefix(line, errPrefix)
		}
	}
	if len(lines) > 0 {
		return lines[0]
	}
	return "failed"
}

func execError(name, value string, traceback []string) *ExecError {
	if traceback == nil {
		traceback = []string{}
	}
	return &ExecError{Name: name, Value: value, Traceback: traceback}
}

// joinLines is the lines as a stream writes them, each ended.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// Complete answers completion at the cursor: the prompt completes a line, so
// the cursor's line is completed, and the span replaced is spelled back in
// the cell's rune offsets.
func (e *REPLEngine) Complete(code string, cursor int) Completion {
	line, lineStart, posInLine := cursorLine(code, cursor)
	c := e.session.Complete(line, posInLine)
	prefixRunes := utf8.RuneCountInString(c.Prefix)
	cursorRunes := utf8.RuneCountInString(code[:lineStart+posInLine])
	return Completion{
		Matches:     c.Candidates,
		CursorStart: cursorRunes - prefixRunes,
		CursorEnd:   cursorRunes,
	}
}

// cursorLine is the line of code the cursor (a rune offset) is on, where that
// line starts in code (a byte offset), and the cursor's byte offset in the line.
func cursorLine(code string, cursor int) (line string, lineStart, posInLine int) {
	at := byteOffset(code, cursor)
	lineStart = strings.LastIndexByte(code[:at], '\n') + 1
	lineEnd := strings.IndexByte(code[at:], '\n')
	if lineEnd < 0 {
		lineEnd = len(code)
	} else {
		lineEnd += at
	}
	return code[lineStart:lineEnd], lineStart, at - lineStart
}

// byteOffset is the byte offset of the rune offset cursor in s, clamped to s.
func byteOffset(s string, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	for i := range s {
		if cursor == 0 {
			return i
		}
		cursor--
	}
	return len(s)
}

// IsComplete reports whether the cell is ready to run: a declaration left open
// by a brace, parenthesis or bracket is not, and its continuation is indented.
func (e *REPLEngine) IsComplete(code string) (IsCompleteStatus, string) {
	stmts := repl.Statements(code)
	if len(stmts) == 0 {
		return Complete, ""
	}
	last := stmts[len(stmts)-1]
	if !last.Meta && repl.NeedsContinuation(last.Text) {
		return Incomplete, "  "
	}
	return Complete, ""
}

// Inspect describes the name under the cursor as %print does.
func (e *REPLEngine) Inspect(code string, cursor int, detail int) Inspection {
	name := nameAt(code, byteOffset(code, cursor))
	if name == "" {
		return Inspection{}
	}
	lines, _, err := e.session.RunMeta("%print " + name)
	if err != nil || failed(lines) || len(lines) == 0 {
		return Inspection{}
	}
	return Inspection{Found: true, Data: textBundle(lines)}
}

// nameAt is the qualified name the byte offset at sits in or just after: the
// identifier runes around it and the `::` joining them.
func nameAt(code string, at int) string {
	isName := func(r rune) bool { return r == '_' || r == ':' || unicode.IsLetter(r) || unicode.IsDigit(r) }
	start := at
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(code[:start])
		if !isName(r) {
			break
		}
		start -= size
	}
	end := at
	for end < len(code) {
		r, size := utf8.DecodeRuneInString(code[end:])
		if !isName(r) {
			break
		}
		end += size
	}
	return strings.Trim(code[start:end], ":")
}

// Interrupt stops the statement running.
func (e *REPLEngine) Interrupt() { e.session.Interrupt() }

// Shutdown has nothing to release: the session is the process's.
func (e *REPLEngine) Shutdown(bool) {}

// String names the engine.
func (e *REPLEngine) String() string { return fmt.Sprintf("REPLEngine(%p)", e.session) }
