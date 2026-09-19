package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// needsContinuation reports whether buf has unbalanced open brackets, i.e. the
// REPL should keep reading on a secondary prompt. It counts {, (, [ against
// their closers, clamping each counter at 0 so stray closers don't force
// continuation. Trailing-operator heuristics (::, =) are deferred.
func needsContinuation(buf string) bool {
	lx := lexer.New(source.New("<repl-input>", []byte(buf)))
	var brace, paren, bracket int
	for {
		tok := lx.Next()
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
