package symbolfacts

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// extractMetadata populates the metadata map from the symbol's AST node.
// Extracts: multiplicity, type, direction, abstract, variation, variant.
func extractMetadata(sym *symbols.Symbol, meta map[string]string) {
	extractVariability(sym, meta)
	if sym.Decl == nil {
		return
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		usageMetadata(decl, meta)
	case *ast.SubjectMember:
		// A subject written in a requirement body: `subject s : T;`.
		if decl.Multiplicity != nil {
			meta["multiplicity"] = formatMultiplicity(decl.Multiplicity)
		}
		if decl.TypeRef != nil {
			meta["type"] = formatQualifiedName(decl.TypeRef)
		}
	case *ast.PrefixMetadata:
		// The metadata definition an `@M` or `#M` annotation is typed by.
		if decl.Type != nil {
			meta["type"] = formatQualifiedName(decl.Type)
		}
	case *ast.Definition:
		definitionMetadata(decl, meta)
	}
}

// usageMetadata states a usage's multiplicity, type, direction and abstractness.
func usageMetadata(decl *ast.Usage, meta map[string]string) {
	if decl.Multiplicity != nil {
		meta["multiplicity"] = formatMultiplicity(decl.Multiplicity)
	}
	// Type (first typing relationship), with the `~` of a conjugated typing
	if rel, qn := firstNamedTarget(decl.Relationships, ast.RelTyping); qn != nil {
		meta["type"] = formatQualifiedName(qn)
		if rel.Conjugated {
			meta["type"] = "~" + meta["type"]
		}
	}
	if decl.Direction != ast.DirNone {
		meta["direction"] = decl.Direction.String()
	}
	if decl.IsAbstract {
		meta["abstract"] = "true"
	}
}

// definitionMetadata states a definition's abstractness and its first specialization.
func definitionMetadata(decl *ast.Definition, meta map[string]string) {
	if decl.IsAbstract {
		meta["abstract"] = "true"
	}
	if _, qn := firstNamedTarget(decl.Relationships, ast.RelSpecializes); qn != nil {
		meta["specializes"] = formatQualifiedName(qn)
	}
}

// firstNamedTarget is the first relationship of kind whose target is a qualified name.
func firstNamedTarget(rels []*ast.Relationship, kind ast.RelationshipKind) (*ast.Relationship, *ast.QualifiedName) {
	for _, rel := range rels {
		if rel.Kind != kind {
			continue
		}
		if qn, ok := rel.Target.(*ast.QualifiedName); ok {
			return rel, qn
		}
	}
	return nil, nil
}

// formatMultiplicity renders Multiplicity as "lower..upper" or "value".
// sendMetadata states a send action's arguments as written (SysML v2
// §8.3.17.15 SendActionUsage): "payload", "sender" (`via`) and "receiver"
// (`to`), each present only when the send writes it.
func (sc *Context) sendMetadata(sym *symbols.Symbol, meta map[string]string) {
	send := symbols.SendStatementOf(sym.Decl)
	if send == nil || sym.Kind != symbols.SymbolSendActionUsage {
		return
	}
	arguments := map[string]ast.Node{"payload": send.Message}
	if send.IsVia {
		arguments["sender"], arguments["receiver"] = send.Target, send.Receiver
	} else {
		arguments["receiver"] = send.Target
	}
	for key, node := range arguments {
		if text := sc.expressionText(sym, node); text != "" {
			meta[key] = text
		}
	}
}

// expressionText is the notation of an expression node of sym's document: its source
// text, or the name a bare reference writes where no source text is at hand.
func (sc *Context) expressionText(sym *symbols.Symbol, node ast.Node) string {
	if node == nil {
		return ""
	}
	if lookup := sc.Semantics.SourceText(); lookup != nil {
		if text := strings.TrimSpace(lookup(sym.DocName, node.Span())); text != "" {
			return text
		}
	}
	return ast.QualifiedText(node)
}

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

// extractVariability records a variation point (`variation`, and `abstract`,
// since a variation is always abstract) and a variant (`variant`); an
// enumeration definition is a variation and its values are variants (SysML v2 §8.3).
func extractVariability(sym *symbols.Symbol, meta map[string]string) {
	if semantics.IsVariation(sym) {
		meta["variation"] = "true"
		meta["abstract"] = "true"
	}
	if semantics.IsVariant(sym) {
		meta["variant"] = "true"
	}
}
