package notebook

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Statement is one unit of a cell: a `%` command line, or a run of declaration
// or expression text the kernel submits whole.
type Statement struct {
	// Command is set when Text is a `%` command line.
	Command bool
	Text    string
	// Line is the line of the cell, counted from 1, that Text begins on.
	Line int
}

// Statements splits a cell of input into what the kernel reads from it. A `%`
// line outside a continuation is a command; an expression that closes its
// brackets is answered on its own, as the prompt answers one line;
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
			if IsCommand(line) {
				flush(&decls, declsAt)
				out = append(out, Statement{Command: true, Text: strings.TrimSpace(line), Line: chunkAt})
				continue
			}
		} else {
			chunk.WriteByte('\n')
		}
		chunk.WriteString(line)
		if NeedsContinuation(chunk.String()) {
			continue
		}
		if _, ok := BareExpression(chunk.String()); ok {
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

// IsCommand reports whether line is a `%` command line.
func IsCommand(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "%")
}

// NeedsContinuation reports whether src is an unfinished submission: a brace,
// parenthesis or bracket it opens is not yet closed, or it ends inside a
// string, quoted name or comment, so the prompt reads another line before
// submitting it. Each counter clamps at 0 so a stray closer forces nothing.
func NeedsContinuation(src string) bool {
	lx := lexer.New(source.New("<cell>", []byte(src)))
	var brace, paren, bracket int
	for {
		tok := lx.Next()
		if tok.Unterminated {
			return true
		}
		switch tok.Kind {
		case lexer.LBrace:
			brace++
		case lexer.RBrace:
			if brace > 0 {
				brace--
			}
		case lexer.LParen:
			paren++
		case lexer.RParen:
			if paren > 0 {
				paren--
			}
		case lexer.LBracket:
			bracket++
		case lexer.RBracket:
			if bracket > 0 {
				bracket--
			}
		case lexer.EOF:
			return brace > 0 || paren > 0 || bracket > 0
		}
	}
}

// BareExpression reports whether src reads as an expression the kernel answers
// rather than as a declaration it submits, and the expression it reads as: text
// that is no file but parses whole as one expression.
func BareExpression(src string) (string, bool) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", false
	}
	fp := parser.New(source.New("<cell>", []byte(src)))
	fp.ParseFile()
	if len(fp.Diagnostics) == 0 {
		return "", false
	}
	p := parser.New(source.New("<cell>", []byte(trimmed)))
	if expr := p.ParseExpression(); expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(trimmed) {
		return "", false
	}
	return trimmed, true
}

// Skipped counts the lines of a cell Declarations blanked.
type Skipped struct {
	CommandLines    int
	ExpressionLines int
}

// Declarations is the text of a cell as what it declares: its declarations
// where they stand, every `%` command line and expression line blank, so a
// position in the result is the same position in the cell.
func Declarations(cell string) (string, Skipped) {
	var skipped Skipped
	lines := make([]string, strings.Count(cell, "\n")+1)
	for _, stmt := range Statements(cell) {
		count := strings.Count(stmt.Text, "\n") + 1
		switch {
		case stmt.Command:
			skipped.CommandLines += count
		case isExpression(stmt.Text):
			skipped.ExpressionLines += count
		default:
			copy(lines[stmt.Line-1:], strings.Split(stmt.Text, "\n"))
		}
	}
	return strings.Join(lines, "\n"), skipped
}

// isExpression reports whether a statement Statements split off is an
// expression the kernel answers, not a declaration it submits.
func isExpression(text string) bool {
	_, ok := BareExpression(text)
	return ok
}
