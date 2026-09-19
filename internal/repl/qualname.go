package repl

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parsedName is what parseName reads from a command's name text.
type parsedName struct {
	name string
	// qualified reports a name of several segments; one segment holding `::`
	// inside its quotes is not.
	qualified bool
	ok        bool
}

// plainName is a name spelled as the symbol index registers it: the notation
// parsed, so a quoted segment ('My Pkg') keeps its text and loses its quotes.
// It reports false for text that is no name, which the caller then uses as
// typed.
func plainName(text string) (string, bool) {
	p := parseName(text)
	return p.name, p.ok
}

// parseName reads a name as plainName does, and whether it was qualified.
func parseName(text string) parsedName {
	text = strings.TrimSpace(text)
	if text == "" {
		return parsedName{}
	}
	p := parser.New(source.New("name", []byte(text)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return parsedName{}
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global {
		return parsedName{}
	}
	name := qualifiedText(ref.Name)
	return parsedName{name: name, qualified: len(ref.Name.Parts) > 1, ok: name != ""}
}

// qualifiedText spells a qualified name as the index registers it, "" for one
// with an empty segment.
func qualifiedText(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	segments := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		if part.Text == "" {
			return ""
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::")
}

// notationName is a qualified name spelled as the notation writes it, so a name
// the prompt prints can be typed back into a command. It is the one rule every
// surface quotes with, `%render` included.
func notationName(fqn string) string {
	return source.QualifiedNameText(fqn)
}

// declaredName spells the name an object is held under from its declaration, segment by
// segment: the flat name alone cannot tell a `::` inside a quoted segment from a qualification.
func (s *Session) declaredName(fqn string) string {
	if idx := s.browseIndex(); idx != nil {
		if sym := idx.Declaring(fqn); sym != nil {
			return declarationNotation(sym)
		}
	}
	return notationName(fqn)
}

// declarationNotation spells a declaration's qualified name as the notation
// writes it, each owner and the name itself quoted on its own where needed.
func declarationNotation(sym *symbols.Symbol) string {
	segments := []string{source.NameText(sym.Name)}
	for scope := sym.OwnerScope; scope != nil && scope.Owner() != nil; scope = scope.Owner().OwnerScope {
		segments = append(segments, source.NameText(scope.Owner().Name))
	}
	slices.Reverse(segments)
	return strings.Join(segments, "::")
}
