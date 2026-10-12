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
	// Line is the line of the cell, counted from 1, that Text begins on.
	Line int
}

// Statements splits a cell of input into what the prompt would read from it.
// A `%` line outside a continuation is a meta command; an expression that
// closes its brackets is answered on its own, as the prompt answers one line;
// declarations run together into one submission up to the next command or
// expression, so a cell may hold a whole model as a file does, blank lines
// included.
func Statements(cell string) []Statement {
	var out []Statement
	var decls, chunk strings.Builder
	declsAt, chunkAt := 0, 0
	flush := func(b *strings.Builder, at int) {
		if strings.TrimSpace(b.String()) != "" {
			out = append(out, Statement{Text: b.String(), Line: at})
		}
		b.Reset()
	}
	// A chunk joins to the declarations before it with a newline, so a blank
	// line keeps its place and the declarations' line is their first text's.
	join := func() {
		if decls.Len() == 0 {
			declsAt = chunkAt
		} else {
			decls.WriteByte('\n')
		}
		decls.WriteString(chunk.String())
		chunk.Reset()
	}
	for i, line := range strings.Split(cell, "\n") {
		if chunk.Len() == 0 {
			chunkAt = i + 1
			if isMeta(line) {
				flush(&decls, declsAt)
				out = append(out, Statement{Meta: true, Text: strings.TrimSpace(line), Line: chunkAt})
				continue
			}
		} else {
			chunk.WriteByte('\n')
		}
		chunk.WriteString(line)
		if needsContinuation(chunk.String()) {
			continue
		}
		if _, ok := bareExpression(chunk.String()); ok {
			flush(&decls, declsAt)
			flush(&chunk, chunkAt)
			continue
		}
		join()
	}
	if chunk.Len() > 0 {
		join()
	}
	flush(&decls, declsAt)
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
