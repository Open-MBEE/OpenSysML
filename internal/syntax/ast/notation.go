package ast

// Notation names a declaration the way the notation writes it — "part def",
// "datatype", "assert constraint" — and is empty for a node the notation has no
// keyword of its own for.
func Notation(node Node) string {
	switch decl := node.(type) {
	case *Definition:
		kw := writtenKeyword(decl.Keyword, decl.Kind.String())
		// A KerML classifier is written without the `def` a SysML definition takes.
		if !decl.HasDefKeyword {
			return kw
		}
		return kw + " def"
	case *Usage:
		kw := writtenKeyword(decl.Keyword, decl.Kind.String())
		if decl.PrefixKeyword != "" {
			return decl.PrefixKeyword + " " + kw
		}
		return kw
	case *RelationshipMember:
		// Only `specialization Spec subtype C :> S` can be named, so the prefix is
		// the keyword the name follows; without one the kind keyword stands alone.
		if decl.PrefixKeyword != "" {
			return decl.PrefixKeyword
		}
		return writtenKeyword(decl.Keyword, decl.Kind.String())
	case *SubstateMember:
		return "state"
	case *SubjectMember:
		return "subject"
	case *AssumeMember:
		return "assume constraint"
	case *RequireMember:
		return "require constraint"
	case *TransitionMember:
		return "transition"
	case *PseudostateNode:
		return decl.Keyword
	case *DecisionNode:
		return "decide"
	case *ForkNode:
		return "fork"
	case *JoinNode:
		return "join"
	case *MergeNode:
		return "merge"
	case *SendStatement:
		return "send"
	case *Documentation:
		return "doc"
	case *TextualRepresentation:
		return "rep"
	}
	return ""
}

// writtenKeyword prefers the keyword a declaration was written with, since
// several spellings (`datatype`, `feature`, `attribute`) share one kind.
func writtenKeyword(written, kind string) string {
	if written != "" {
		return written
	}
	return kind
}
