package ast

import "strings"

// Notation names a declaration the way the notation writes it — "part def",
// "datatype", "assert constraint" — and is empty for a node the notation has no
// keyword of its own for.
func Notation(node Node) string {
	switch decl := node.(type) {
	case *Definition:
		kw := writtenKeyword(decl.Keyword, decl.Kind.String())
		// A KerML classifier is written without the `def` a SysML definition takes.
		if decl.HasDefKeyword {
			kw += " def"
		}
		return modifierPrefixed(kw, decl.IsAbstract, decl.IsVariation)
	case *Usage:
		kw := writtenKeyword(decl.Keyword, decl.Kind.String())
		if decl.PrefixKeyword != "" {
			kw = decl.PrefixKeyword + " " + kw
		}
		// `variant part x` keeps its kind keyword; the shorthand `variant x` is written `variant` alone.
		kw = modifierPrefixed(kw, decl.IsAbstract, decl.IsVariation)
		if decl.IsVariant && decl.Keyword != "variant" && decl.PrefixKeyword != "variant" {
			kw = "variant " + kw
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

// modifierPrefixed writes the modifier before the kind keyword: `variation`, which
// says abstract too so `abstract` is never written on one (§7.6.7); else `abstract`.
func modifierPrefixed(kw string, isAbstract, isVariation bool) string {
	switch {
	case isVariation:
		return "variation " + kw
	case isAbstract:
		return "abstract " + kw
	}
	return kw
}

// NotationKeyword is the kind keyword a notation names after its modifiers:
// `part def` of `abstract part def`, `ref part` of `variant ref part`, `variant` alone of `variant`.
func NotationKeyword(notation string) string {
	kw, rest, _ := strings.Cut(notation, " ")
	for rest != "" && (kw == "abstract" || kw == "variation" || kw == "variant") {
		kw, rest, _ = strings.Cut(rest, " ")
	}
	if rest == "" {
		return kw
	}
	return kw + " " + rest
}

// writtenKeyword prefers the keyword a declaration was written with, since
// several spellings (`datatype`, `feature`, `attribute`) share one kind.
func writtenKeyword(written, kind string) string {
	if written != "" {
		return written
	}
	return kind
}
