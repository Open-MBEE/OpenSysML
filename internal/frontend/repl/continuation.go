package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// openLiteral reports whether buf ends inside a string, quoted name, comment or
// note still waiting for its closer, so a blank line typed next is its content
// rather than a request to submit.
func openLiteral(buf string) bool {
	lx := lexer.New(source.New("<repl-input>", []byte(buf)))
	for {
		tok := lx.Next()
		if tok.Unterminated {
			return true
		}
		if tok.Kind == lexer.EOF {
			return false
		}
	}
}
