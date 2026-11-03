package lexer

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// NotASingleName is NameProblem's answer to text that starts with a name
// but carries more: a type, a multiplicity, a second word.
const NotASingleName = "is not a single name"

// NameProblem says why text is not one declared name — a basic identifier or
// a quoted unrestricted name, and not a keyword — or "" when it is one.
func NameProblem(name string) string {
	if name == "" {
		return "is empty"
	}
	if source.IsKeyword(name) {
		return "is a keyword"
	}
	tok := New(source.New("<name>", []byte(name))).Next()
	switch {
	case tok.Kind != Identifier && tok.Kind != UnrestrictedName:
		return "is not an identifier"
	case tok.Unterminated:
		return "is an unterminated quoted name"
	case tok.Span.Len != len(name):
		return NotASingleName
	}
	return ""
}
