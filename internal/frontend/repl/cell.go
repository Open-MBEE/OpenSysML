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

// Statements splits a cell of input into what the prompt would read from it.
// A `%` line outside a continuation is a meta command; an expression that
// closes its brackets is answered on its own, as the prompt answers one line;
// declarations run together into one submission up to the next command or
// expression, so a cell may hold a whole model as a file does, blank lines
// included.
func Statements(cell string) []Statement {
	var out []Statement
	var decls, chunk strings.Builder
	flush := func(b *strings.Builder) {
		if strings.TrimSpace(b.String()) != "" {
			out = append(out, Statement{Text: b.String()})
		}
		b.Reset()
	}
	for _, line := range strings.Split(cell, "\n") {
		if chunk.Len() == 0 && isMeta(line) {
			flush(&decls)
			out = append(out, Statement{Meta: true, Text: strings.TrimSpace(line)})
			continue
		}
		if chunk.Len() > 0 {
			chunk.WriteByte('\n')
		}
		chunk.WriteString(line)
		if needsContinuation(chunk.String()) {
			continue
		}
		if _, ok := bareExpression(chunk.String()); ok {
			flush(&decls)
			flush(&chunk)
			continue
		}
		if decls.Len() > 0 {
			decls.WriteByte('\n')
		}
		decls.WriteString(chunk.String())
		chunk.Reset()
	}
	if chunk.Len() > 0 {
		if decls.Len() > 0 {
			decls.WriteByte('\n')
		}
		decls.WriteString(chunk.String())
	}
	flush(&decls)
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
