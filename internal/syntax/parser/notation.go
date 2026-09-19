package parser

// The words OpenSysML's own state and action notation uses are a literal in
// none of the pinned grammars (docs/reference/grammar/conformance-audit.md), so
// the lexer does not reserve them: they arrive as names and are matched here by
// the shape of the notation around them, as `point` and `var` are.

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// notationWords are the unreserved words our notation matches contextually.
var notationWords = map[string]bool{
	"choice":   true,
	"deep":     true,
	"defer":    true,
	"done":     true,
	"history":  true,
	"junction": true,
	"shallow":  true,
}

// sysmlOnlyWords are literals of SysML.xtext that appear in neither
// KerML.xtext nor KerMLExpressions.xtext: `at` (`SysML.xtext:1480`, an accept
// trigger), `while` (`:1617`, a loop action), `merge` (`:1666`) and `decide`
// (`:1672`, control nodes) and the portion modifiers `snapshot`/`timeslice`
// (`:864`, PortionKind). A word the file's own grammar does not reserve is a
// name there, so in a `.kerml` file these read as ordinary names.
var sysmlOnlyWords = map[string]bool{
	"at":        true,
	"while":     true,
	"merge":     true,
	"decide":    true,
	"snapshot":  true,
	"timeslice": true,
}

// unreserved reclassifies a keyword the file's grammar does not reserve as the
// name it is, so every position that takes a name accepts it.
func (p *Parser) unreserved(tok lexer.Token) lexer.Token {
	if tok.Kind != lexer.Keyword || p.src.Kind() != source.KindKerML || !sysmlOnlyWords[tok.KeywordID] {
		return tok
	}
	tok.Kind = lexer.Identifier
	tok.KeywordID = ""
	return tok
}

// notationWordAt returns the notation word n tokens ahead: a keyword's identity,
// or one of the unreserved words above, and "" for anything else.
func (p *Parser) notationWordAt(n int) string {
	t := p.peekN(n)
	switch t.Kind {
	case lexer.Keyword:
		return t.KeywordID
	case lexer.Identifier:
		if w := p.src.Text(t.Span); notationWords[w] {
			return w
		}
	}
	return ""
}

// peekIsName reports whether the token n ahead can be a declared name.
func (p *Parser) peekIsName(n int) bool {
	switch p.peekN(n).Kind {
	case lexer.Identifier, lexer.Keyword, lexer.UnrestrictedName:
		return true
	}
	return false
}

// atActionNodeWord returns the unreserved word at the cursor when it heads an
// action node — `done;`, `done end;` — rather than naming a feature.
func (p *Parser) atActionNodeWord() (string, bool) { return p.actionNodeWordAt(0) }

// actionNodeWordAt recognizes a named `done` spelling so parseFinalNode can
// report its diagnostic instead of cascading.
func (p *Parser) actionNodeWordAt(n int) (string, bool) {
	if p.peekN(n).Kind != lexer.Identifier {
		return "", false
	}
	w := p.src.Text(p.peekN(n).Span)
	if w != "done" {
		return "", false
	}
	if p.peekN(n+1).Kind == lexer.Semicolon {
		return w, true
	}
	if !p.peekIsName(n + 1) {
		return "", false
	}
	switch next := p.peekN(n + 2); next.Kind {
	case lexer.Semicolon:
		return w, true
	case lexer.Keyword:
		// A `then` or a guard after the name is still the node's shape, so the
		// node parser diagnoses it rather than the word reading as an edge end.
		if next.KeywordID == "then" || next.KeywordID == "if" {
			return w, true
		}
	}
	return "", false
}

// atStateNotationWord returns the unreserved word at the cursor when it heads a
// state body member of our own notation rather than naming a feature.
func (p *Parser) atStateNotationWord() (string, bool) {
	if p.peek().Kind != lexer.Identifier {
		return "", false
	}
	w := p.src.Text(p.peek().Span)
	switch w {
	case "choice", "junction", "history":
		// `<word> <name>;`
		if p.peekIsName(1) && p.peekN(2).Kind == lexer.Semicolon {
			return w, true
		}
	case "shallow", "deep":
		// `<word> history <name>;`
		if p.notationWordAt(1) == "history" && p.peekIsName(2) && p.peekN(3).Kind == lexer.Semicolon {
			return w, true
		}
	case "defer":
		// `defer <event> [, <event>]*;`, the event parsed as a trigger is a name
		// or a call, so anything else after the word names a feature.
		if p.peekIsName(1) {
			return w, true
		}
	}
	return "", false
}
