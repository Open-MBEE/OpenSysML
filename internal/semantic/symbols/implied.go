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
	// ImpliedDefaultMultiplicity is the Multiplicity stating a usage's implicit [1..1]
	// (SysML v2 §7.6.3), owned last among the usage's memberships; it declares no name.
	ImpliedDefaultMultiplicity
	// ImpliedMultiplicityRange is the MultiplicityRange a usage's written range (`[2]`)
	// declares, owned through an OwningMembership at the clause; it declares no name.
	ImpliedMultiplicityRange
	// ImpliedSuccessionSourceEnd and ImpliedSuccessionTargetEnd are the nameless end features a `then x;`
	// succession owns for its source and target (SysML.xtext TargetSuccession), named by the end they redefine.
	ImpliedSuccessionSourceEnd
	ImpliedSuccessionTargetEnd
	// ImpliedTransitionSuccessionSourceEnd and ImpliedTransitionSuccessionTargetEnd are the same two ends of the
	// succession a transition owns for its `then` clause (SysML.xtext TransitionSuccessionMember), under the transition.
	ImpliedTransitionSuccessionSourceEnd
	ImpliedTransitionSuccessionTargetEnd
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

// Span is the span of the written declaration the element is implied for; empty
// when that declaration is a record, which has no node.
func (d *ImpliedDecl) Span() source.Span {
	if d.Of == nil {
		return source.Span{}
	}
	return d.Of.Span()
}

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

// ImpliedSuccessionEndIndex is the position, 0 or 1, of the `then` succession end sym implies.
func ImpliedSuccessionEndIndex(sym *Symbol) (int, bool) {
	d, ok := ImpliedDeclOf(sym)
	if !ok {
		return 0, false
	}
	switch d.Form {
	case ImpliedSuccessionSourceEnd, ImpliedTransitionSuccessionSourceEnd:
		return 0, true
	case ImpliedSuccessionTargetEnd, ImpliedTransitionSuccessionTargetEnd:
		return 1, true
	}
	return 0, false
}

// ImpliedTransitionSuccessionEnd reports whether sym is an end of the succession a transition owns,
// which the conversion mints under the transition's succession rather than under a written edge.
func ImpliedTransitionSuccessionEnd(sym *Symbol) bool {
	d, ok := ImpliedDeclOf(sym)
	return ok && (d.Form == ImpliedTransitionSuccessionSourceEnd || d.Form == ImpliedTransitionSuccessionTargetEnd)
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
