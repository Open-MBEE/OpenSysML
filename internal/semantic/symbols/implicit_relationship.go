package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// ImplicitForm is the kind of relationship an ImplicitRelationship reflects:
// a clause written on its owner, or a Membership, Annotation or Import the
// abstract syntax implies for a declaration the notation writes without one.
type ImplicitForm uint8

const (
	// ImplicitWritten reflects a relationship clause written on Owner
	// (`:> A`, `: T`, `subsets f`, …).
	ImplicitWritten ImplicitForm = iota
	// ImplicitMembership reflects the OwningMembership through which Owner, a
	// namespace, owns Member (KerML 8.3.2.5).
	ImplicitMembership
	// ImplicitOwnedAnnotation reflects an Annotation Owner owns as the
	// annotated element, whose annotating element Member it owns through it
	// (`#M` prefix metadata, an annotating element in a relationship body).
	ImplicitOwnedAnnotation
	// ImplicitAnnotatingRelationship reflects an Annotation the annotating
	// element Owner owns for its `about` clause naming Member.
	ImplicitAnnotatingRelationship
	// ImplicitImport reflects the Import declared by Decl in the namespace Owner.
	ImplicitImport
)

// ImplicitRelationship is a relationship the abstract syntax holds as an
// element of its own but the notation writes as part of its owner (KerML
// 8.3.2, 8.3.3). The symbol that carries it is synthesized by the reflective
// layer and is never registered in a scope or index.
type ImplicitRelationship struct {
	Owner      *Symbol              // the element the relationship is written on (owningRelatedElement)
	Ordinal    int                  // position among Owner's relationships of the same Form
	Kind       ast.RelationshipKind // the notation's relationship kind (ImplicitWritten)
	Conjugated bool                 // the `~` of a conjugation or conjugated typing
	Node       *ast.Relationship    // the written edge; nil when Owner is recorded
	Form       ImplicitForm         // what the relationship is
	Member     *Symbol              // the element at the other end of a membership or annotation
	Decl       ast.Node             // the import declaration of an ImplicitImport
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

// ImplicitWrittenRelationship reports whether sym reflects a relationship
// clause written on its owner, rather than an implied membership, annotation
// or import.
func (s *Symbol) ImplicitWrittenRelationship() bool {
	return s != nil && s.Implicit != nil && s.Implicit.Form == ImplicitWritten
}
