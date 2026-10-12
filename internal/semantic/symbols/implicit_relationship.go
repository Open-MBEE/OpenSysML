package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// ImplicitRelationship is a relationship written as notation on its owning
// element, reflected as an element of its own (KerML 8.3.3). The symbol that
// carries it is synthesized by the reflective layer and is never registered
// in a scope or index.
type ImplicitRelationship struct {
	Owner      *Symbol              // the element the relationship is written on (owningRelatedElement)
	Ordinal    int                  // position among Owner's written relationships
	Kind       ast.RelationshipKind // the notation's relationship kind
	Conjugated bool                 // the `~` of a conjugation or conjugated typing
	Node       *ast.Relationship    // the written edge; nil when Owner is recorded
}

// ImplicitOrdinal is the position sym's written relationship holds among its
// owner's relationships, or -1 for an ordinary symbol. It keeps two
// relationships of one recorded owner, which share a declaration span, from
// ordering equal.
func (s *Symbol) ImplicitOrdinal() int {
	if s == nil || s.Implicit == nil {
		return -1
	}
	return s.Implicit.Ordinal
}
