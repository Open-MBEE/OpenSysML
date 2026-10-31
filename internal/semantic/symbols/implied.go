package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ImpliedForm is the kind of element an ImpliedDecl declares.
type ImpliedForm uint8

const (
	// ImpliedConjugatedPortDefinition is a PortDefinition's implied conjugate;
	// its effective name is null without a name, else `~` plus that name; it declares no short name.
	ImpliedConjugatedPortDefinition ImpliedForm = iota
)

// ImpliedDecl declares an element the abstract syntax implies for a written
// declaration Of, which the notation has no text for. It is created by the
// symbol builder beside the written declaration's symbol and is never a parse
// result: the AST stays what the parser produced, and the implied element is a
// derived fact keyed to its written source.
type ImpliedDecl struct {
	Of   ast.Node
	Form ImpliedForm
}

// Span is the span of the written declaration the element is implied for.
func (d *ImpliedDecl) Span() source.Span { return d.Of.Span() }

// LeadingTrivia is empty: nothing is written for an implied element.
func (d *ImpliedDecl) LeadingTrivia() []ast.Trivia { return nil }

// TrailingTrivia is empty: nothing is written for an implied element.
func (d *ImpliedDecl) TrailingTrivia() []ast.Trivia { return nil }

// ImpliedDeclOf returns the implied declaration sym carries, if any.
func ImpliedDeclOf(sym *Symbol) (*ImpliedDecl, bool) {
	if sym == nil {
		return nil, false
	}
	d, ok := sym.Decl.(*ImpliedDecl)
	return d, ok
}

// Implied reports whether sym declares an element the abstract syntax implies
// rather than one the notation writes: no text, diagram or outline shows it.
func (s *Symbol) Implied() bool {
	_, ok := ImpliedDeclOf(s)
	return ok || (s.Recorded() && s.Facts.Node == NodeConjugatedPortDef)
}

// IsConjugatedPortDefinition reports whether sym is the ConjugatedPortDefinition
// a port definition implies, parsed or recorded.
func (s *Symbol) IsConjugatedPortDefinition() bool {
	if s == nil {
		return false
	}
	if s.Recorded() {
		return s.Facts.Node == NodeConjugatedPortDef
	}
	d, ok := s.Decl.(*ImpliedDecl)
	return ok && d.Form == ImpliedConjugatedPortDefinition
}

// buildConjugatedPortDefinition registers the first owned member: its effective
// name is `~` plus the original's name or null; it declares no short name.
func buildConjugatedPortDefinition(scope *Scope, d *ast.Definition) {
	if d.Kind != ast.DefPort || (d.Ident.Name == "" && d.Ident.ShortName == "") {
		return
	}
	decl := &ImpliedDecl{Of: d, Form: ImpliedConjugatedPortDefinition}
	id := ast.Identification{}
	if d.Ident.Name != "" {
		id.Name = "~" + d.Ident.Name
		id.NameSpan = d.Ident.NameSpan
	}
	child := NewScope(scope, decl)
	sym := newSymbol(id, SymbolPortDef, decl, ast.VisibilityDefault, child, scope, nil)
	defineIdent(scope, id, sym)
	scope.AddChild(child)
}
