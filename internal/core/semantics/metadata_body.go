package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The body of a metadata annotation restates features of the annotated metadata
// type: each declaration redefines a feature of that type, implicitly by name or
// explicitly with `:>>`, so one that redefines no such feature is not a body
// feature (KerML 7.4.7, 8.3.3.3, validateMetadataFeatureBody).

// MetadataBodyViolations returns the declarations in the body of the annotation
// that redefine no feature of the metadata type it names, at any nesting depth.
// It returns nothing when the type does not resolve: an unresolved reference is
// reported by name resolution, and every name under it would be a false report.
func (m *Model) MetadataBodyViolations(scope *symbols.Scope, prefix *ast.PrefixMetadata) []ast.Node {
	if m == nil || prefix == nil || prefix.Type == nil || len(prefix.Body) == 0 {
		return nil
	}
	def, ok := m.resolver.ResolveQualified(scope, prefix.Type)
	if !ok || def == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
		def = resolved
	}
	return m.MetadataBodyViolationsOf(def, valueScope(scope, prefix), prefix.Body)
}

// MetadataBodyViolationsOf is MetadataBodyViolations for a body whose metadata
// type is already resolved and whose declarations are registered in scope — the
// body of `metadata m : A { … }` as much as of `@A { … }`.
func (m *Model) MetadataBodyViolationsOf(def *symbols.Symbol, scope *symbols.Scope, body []ast.Node) []ast.Node {
	if m == nil || def == nil || len(body) == 0 {
		return nil
	}
	var out []ast.Node
	m.collectMetadataBodyViolations(def, scope, body, &out)
	return out
}

// MetadataBodyInevaluableValues returns the values written in the body of the
// annotation that are not model-level evaluable, at any nesting depth. A
// metadata feature is a model-level element, so its value must be one the model
// alone decides (KerML 7.4.7, Expression::isModelLevelEvaluable), except one the
// run reads by design (RunDecidedMetadataFeature).
func (m *Model) MetadataBodyInevaluableValues(scope *symbols.Scope, prefix *ast.PrefixMetadata) []ast.Node {
	if m == nil || prefix == nil || len(prefix.Body) == 0 {
		return nil
	}
	return m.MetadataBodyInevaluableValuesOf(m.resolveMetadataType(scope, prefix.Type), valueScope(scope, prefix), prefix.Body)
}

// MetadataBodyInevaluableValuesOf is MetadataBodyInevaluableValues for a body whose
// declarations are registered in scope, such as that of `metadata m : A { … }`;
// def is the metadata type, nil when it does not resolve.
func (m *Model) MetadataBodyInevaluableValuesOf(def *symbols.Symbol, scope *symbols.Scope, body []ast.Node) []ast.Node {
	if m == nil || len(body) == 0 {
		return nil
	}
	var out []ast.Node
	m.collectInevaluableValues(def, scope, body, &out)
	return out
}

// valueScope is the scope a body's values resolve in: the one built for it, which
// sees the metadata type's members before the annotated element's, else scope.
func valueScope(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if scope == nil {
		return nil
	}
	if child := scope.ChildFor(node); child != nil {
		return child
	}
	return scope
}

// collectInevaluableValues walks a metadata annotation body, collecting the
// values of its declarations that the model cannot evaluate; owner is the type
// whose features the level restates, nil when unknown.
func (m *Model) collectInevaluableValues(owner *symbols.Symbol, scope *symbols.Scope, body []ast.Node, out *[]ast.Node) {
	for _, node := range body {
		usage := metadataBodyFeature(node)
		if usage == nil {
			continue
		}
		target := m.metadataBodyTargetOf(owner, scope, usage)
		if usage.Value != nil && !RunDecidedMetadataFeature(owner, target) && !m.ModelLevelEvaluable(scope, usage.Value) {
			*out = append(*out, usage.Value)
		}
		m.collectInevaluableValues(target, valueScope(scope, usage), usage.Members, out)
	}
}

// metadataBodyTargetOf is the feature of owner a body declaration restates, by
// `:>>` or by name; nil when owner is unknown or it restates none.
func (m *Model) metadataBodyTargetOf(owner *symbols.Symbol, scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	if owner == nil {
		return nil
	}
	if declaresRedefinitionAST(usage) {
		redefined, _ := m.metadataBodyRedefinition(owner, scope, usage)
		return redefined
	}
	return symbols.MetadataBodyTarget(m, owner, usage.Ident)
}

// collectMetadataBodyViolations walks one body level against the features owner
// offers, descending into the body of each declaration that does redefine one.
func (m *Model) collectMetadataBodyViolations(owner *symbols.Symbol, scope *symbols.Scope, body []ast.Node, out *[]ast.Node) {
	if owner == nil {
		return
	}
	for _, node := range body {
		usage := metadataBodyFeature(node)
		if usage == nil {
			continue
		}
		var target *symbols.Symbol
		if declaresRedefinitionAST(usage) {
			redefined, resolved := m.metadataBodyRedefinition(owner, scope, usage)
			if !resolved {
				continue // name resolution reports the unresolved target
			}
			target = redefined
		} else {
			target = symbols.MetadataBodyTarget(m, owner, usage.Ident)
		}
		if target == nil {
			*out = append(*out, usage)
			continue
		}
		m.collectMetadataBodyViolations(target, valueScope(scope, usage), usage.Members, out)
	}
}

// metadataBodyFeature is the body feature a metadata body member declares, or nil for
// a non-feature member or a metadata feature, which annotates the body's owner instead.
func metadataBodyFeature(node ast.Node) *ast.Usage {
	if mem, ok := node.(*ast.Membership); ok {
		node = mem.Member
	}
	usage, ok := node.(*ast.Usage)
	if !ok || usage.Kind == ast.UsageMetadata {
		return nil
	}
	return usage
}

// metadataBodyRedefinition returns the redefined feature owned by a type owner
// conforms to, or nil when `:>>` names none; false when no target resolves.
func (m *Model) metadataBodyRedefinition(owner *symbols.Symbol, scope *symbols.Scope, usage *ast.Usage) (*symbols.Symbol, bool) {
	declared := memberSymbol(scope, usage)
	if declared == nil {
		return nil, false
	}
	redefined := m.RedefinedFeatures(declared)
	if len(redefined) == 0 {
		return nil, false
	}
	for _, feature := range redefined {
		if featureOwner := m.ownerOf(feature); featureOwner != nil && m.Conforms(owner, featureOwner) {
			return feature, true
		}
	}
	return nil, true
}

// declaresRedefinitionAST reports whether the declaration carries a
// `redefines`/`:>>` clause.
func declaresRedefinitionAST(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}
