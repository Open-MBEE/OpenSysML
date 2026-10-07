package repl

import (
	"slices"
	"strings"
)

// Statement is one unit of a cell: a meta command line, or a run of declaration
// or expression text submitted whole.
type Statement struct {
	// Meta is set when Text is a `%` command line.
	Meta bool
	Text string
}

// Statements splits a cell of input into what the prompt would read from it,
// line by line, as the REPL loop does: a `%` line outside a continuation is a
// meta command, and everything else accumulates until the next meta command or
// the end of the cell. Unlike the prompt, a blank line does not end a
// continuation, so a declaration body may hold one.
func Statements(cell string) []Statement {
	var out []Statement
	var buf strings.Builder
	flush := func() {
		if strings.TrimSpace(buf.String()) != "" {
			out = append(out, Statement{Text: buf.String()})
		}
		buf.Reset()
	}
	for _, line := range strings.Split(cell, "\n") {
		if isMeta(line) && (buf.Len() == 0 || !needsContinuation(buf.String())) {
			flush()
			out = append(out, Statement{Meta: true, Text: strings.TrimSpace(line)})
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	flush()
	return out
}

// NeedsContinuation reports whether src is an unfinished submission: a brace,
// parenthesis or bracket it opens is not yet closed, so the prompt would read
// another line before submitting it.
func NeedsContinuation(src string) bool { return needsContinuation(src) }

// IsMeta reports whether line is a `%` meta command.
func IsMeta(line string) bool { return isMeta(line) }

// BareExpression reports whether src reads as an expression the prompt answers
// rather than as a declaration, and the expression it reads as.
func BareExpression(src string) (string, bool) { return bareExpression(src) }

// RenderResult produces the lines the prompt prints for a submission at the
// given verbosity.
func RenderResult(r Result, v Verbosity) []string { return renderResult(r, v) }

// Failed reports whether a submission was refused or had an error at the given
// verbosity: what the prompt prints for it reports a failure, not a declaration.
func (r Result) Failed(v Verbosity) bool {
	if r.Refused != nil {
		return true
	}
	if v >= VerbosityDebug {
		return hasError(r.Diagnostics)
	}
	return hasError(scopedDiagnostics(r, v))
}

// KnownMeta reports whether name is a `%` command this build serves, so a
// caller can tell a command that printed guidance from one that was unknown.
func KnownMeta(name string) bool {
	i := slices.IndexFunc(metaCommandTable, func(c metaCommand) bool { return c.name == name })
	return i >= 0 && metaCommandTable[i].served()
}

// MetaArgs reads a meta command line as the prompt does: the command, then its
// arguments, a quoted one kept whole.
func MetaArgs(line string) []string { return parseArgs(strings.TrimSpace(line)) }
