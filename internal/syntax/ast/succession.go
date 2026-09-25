package ast

// IsSuccessionSource reports whether a positional `then` sequences from this body
// member: a feature that is not an edge (SysML v2 §7.17.4). Parser and RDF writer share it.
func IsSuccessionSource(member Node) bool {
	switch n := member.(type) {
	case *Membership:
		return n.Member != nil && IsSuccessionSource(n.Member)
	case *Usage:
		return !n.Kind.IsEdge()
	case *SuccessionEdge, *ControlFlowEdge, *ObjectFlowEdge, *TransitionEdge, *TransitionMember:
		return false
	case *Definition, *Package, *Namespace, *Import, *Alias, *Dependency, *MultiplicityDecl,
		*RelationshipMember, *Comment, *Documentation, *TextualRepresentation, *FilterMember,
		*DeferMember:
		return false
	}
	return true
}
