package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// featureDecl is the declaration the typing and value checks read, whichever
// node declares it: a usage, or the constraint an assume/require member owns.
type featureDecl struct {
	node          ast.Node
	kind          ast.UsageKind
	keyword       string
	relationships []*ast.Relationship
	multiplicity  *ast.Multiplicity
	value         ast.Node
	span          source.Span
}

func usageDecl(u *ast.Usage) featureDecl {
	return featureDecl{
		node:          u,
		kind:          u.Kind,
		keyword:       u.Keyword,
		relationships: u.Relationships,
		multiplicity:  u.Multiplicity,
		value:         u.Value,
		span:          u.Span(),
	}
}

// featureDeclOf returns the feature a node declares, or false for a node that
// declares none (a definition, a reference or condition form of assume/require).
func featureDeclOf(n ast.Node) (featureDecl, bool) {
	if u, ok := n.(*ast.Usage); ok {
		return usageDecl(u), true
	}
	oc, ok := ast.OwnedConstraintOf(n)
	if !ok {
		return featureDecl{}, false
	}
	span := oc.DeclSpan
	if span.Len == 0 {
		span = n.Span()
	}
	return featureDecl{
		node:          n,
		kind:          ast.UsageConstraint,
		keyword:       "constraint",
		relationships: oc.Relationships,
		multiplicity:  oc.Multiplicity,
		value:         oc.Value,
		span:          span,
	}, true
}

// hasTyping reports whether the declaration names a type of its own.
func (d featureDecl) hasTyping() bool {
	return hasTypingRelationship(d.relationships)
}
