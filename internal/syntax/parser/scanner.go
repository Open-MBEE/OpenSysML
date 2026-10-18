package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Scanner reads a file's tokens as the parser does. The lexer tokenizes the
// union of both languages' keywords; a keyword the file's own grammar does not
// reserve — a KerML-only literal such as `step` in a `.sysml` file, a SysML-only
// one such as `action` in `.kerml` — is the name it is there, so the Scanner
// hands it on as an Identifier. The one exception is a named declaration of the
// other notation: `step s;` in a `.sysml` file keeps its keyword, so the
// declaration parses and the notation pass can report it, where `step <= bound`,
// `step : Real;`, `step { }` and `(step)` read the name. The rule is the same for
// every consumer of the token stream (parser, semantic tokens), so they classify
// a word alike.
type Scanner struct {
	lx      *lexer.Lexer
	kind    source.Kind
	pending []lexer.Token // tokens scanned ahead of the one being returned, in order
	head    int           // index in pending of the next token to return
}

// NewScanner returns a Scanner over sf.
func NewScanner(sf *source.SourceFile) *Scanner {
	return &Scanner{lx: lexer.New(sf), kind: sf.Kind()}
}

// Next returns the next token, trivia included; EOF is returned repeatedly.
func (s *Scanner) Next() lexer.Token {
	tok := s.take()
	if tok.Kind == lexer.Keyword && !Reserves(s.kind, tok.KeywordID) && s.readsAsName() {
		tok.Kind = lexer.Identifier
		tok.KeywordID = ""
	}
	return tok
}

// take returns the next raw token, from the look-ahead queue before the lexer.
func (s *Scanner) take() lexer.Token {
	if s.head < len(s.pending) {
		tok := s.pending[s.head]
		s.head++
		if s.head == len(s.pending) {
			s.pending, s.head = s.pending[:0], 0
		}
		return tok
	}
	return s.lx.Next()
}

// ahead returns the n-th token after the one being returned, trivia and
// comments skipped, scanning further into the queue as needed.
func (s *Scanner) ahead(n int) lexer.Token {
	seen := 0
	for i := s.head; ; i++ {
		if i == len(s.pending) {
			s.pending = append(s.pending, s.lx.Next())
		}
		t := s.pending[i]
		if t.IsTrivia() || t.Kind == lexer.RegularComment {
			continue
		}
		if seen == n || t.Kind == lexer.EOF {
			return t
		}
		seen++
	}
}

// readsAsName reports whether the unreserved keyword being returned is a name
// rather than the keyword of a declaration in the other notation. Before a
// name, a short name, prefix metadata or a further keyword of a declaration
// (`step s`, `feature redefines x`, `chains a.b`) the word keeps its keyword,
// since a declaration of the other notation is parsed and then reported by the
// notation pass (docs/reference/grammar/conformance-audit.md). Before anything
// else — a body or multiplicity (`step { }`, `step [1]`), an operator, `:`, `;`,
// `=`, `)`, `}`, `,`, `.`, `::`, `(` — the word is the name, since the file's
// own grammar gives it no other reading; `inv { x > 0 }` in a `.sysml` file is
// then the syntax error the pinned pilot reports, not an anonymous invariant.
func (s *Scanner) readsAsName() bool {
	next := s.ahead(0)
	switch next.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return false
	case lexer.Keyword:
		// `step and ready`, `c ? step else other` continue an expression.
		return wordBinaryOpKeywords[next.KeywordID] || next.KeywordID == "else"
	case lexer.Lt:
		// `step <s> x;` opens a short name; `step < bound` and the chained
		// `step < a > b }` compare.
		if !spellsName(s.ahead(1)) || s.ahead(2).Kind != lexer.Gt {
			return true
		}
		after := s.ahead(3)
		if spellsName(after) {
			after = s.ahead(4)
		}
		return !continuesDeclaration(after)
	case lexer.Hash:
		// `step #M s` is prefix metadata; `step#(1)` indexes.
		return s.ahead(1).Kind == lexer.LParen
	}
	return true
}

// continuesDeclaration reports whether t can follow a declaration's name: its
// end, a type, a multiplicity, a body, a value, a specialization or a keyword.
func continuesDeclaration(t lexer.Token) bool {
	switch t.Kind {
	case lexer.Semicolon, lexer.Colon, lexer.LBracket, lexer.LBrace, lexer.Eq, lexer.ColonEq,
		lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt, lexer.Keyword:
		return true
	}
	return false
}

// spellsName reports whether t can spell a declared name, a keyword included.
func spellsName(t lexer.Token) bool {
	switch t.Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
		return true
	}
	return false
}
