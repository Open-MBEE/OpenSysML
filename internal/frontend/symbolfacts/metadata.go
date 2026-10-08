package symbolfacts

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// extractMetadata populates the metadata map from the symbol's AST node.
// Extracts: multiplicity, type, direction, abstract.
func extractMetadata(sym *symbols.Symbol, meta map[string]string) {
	if sym.Decl == nil {
		return
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		// Multiplicity
		if decl.Multiplicity != nil {
			meta["multiplicity"] = formatMultiplicity(decl.Multiplicity)
		}
		// Type (first typing relationship)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelTyping {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["type"] = formatQualifiedName(qn)
					break
				}
			}
		}
		// Direction
		if decl.Direction != ast.DirNone {
			meta["direction"] = decl.Direction.String()
		}
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}
	case *ast.Definition:
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}
		// Type (first specializes relationship for definitions)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelSpecializes {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["specializes"] = formatQualifiedName(qn)
					break
				}
			}
		}
	}
}

// formatMultiplicity renders Multiplicity as "lower..upper" or "value".
func formatMultiplicity(m *ast.Multiplicity) string {
	if !m.IsRange {
		return formatMultiplicityBound(m.Lower)
	}
	lower := formatMultiplicityBound(m.Lower)
	upper := formatMultiplicityBound(m.Upper)
	return lower + ".." + upper
}

// formatMultiplicityBound renders a multiplicity bound node as a string.
func formatMultiplicityBound(n ast.Node) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.LiteralInteger:
		return v.Value
	case *ast.LiteralInfinity:
		return "*"
	default:
		return "?"
	}
}

// formatQualifiedName renders QualifiedName as "A::B::C".
func formatQualifiedName(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return joinParts(parts, "::")
}

// joinParts joins parts with separator.
func joinParts(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

// visibilityToString converts ast.Visibility to string.
func visibilityToString(v ast.Visibility) string {
	switch v {
	case ast.VisibilityPublic:
		return "public"
	case ast.VisibilityPrivate:
		return "private"
	case ast.VisibilityProtected:
		return "protected"
	case ast.VisibilityDefault:
		return "default"
	default:
		return "default"
	}
}
