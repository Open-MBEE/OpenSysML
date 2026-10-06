package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ImpliedEndReference is the feature a connector end attaches to without a
// `references` clause of its own saying so (`connect s.y to k.u`), which it
// reference-subsets all the same (SysML v2 8.2.2.13.1); nil when a clause does.
func ImpliedEndReference(end *ast.ConnectorEnd) ast.Node {
	target := end.AttachedTarget()
	if target == nil {
		return nil
	}
	for _, rel := range end.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences && rel.Target == target {
			return nil
		}
	}
	return target
}

// connectorEndAttachment is the reference subsetting an end's attachment
// states, as the written edge the relationship object it reflects spans.
func connectorEndAttachment(target ast.Node) *ast.Relationship {
	rel := &ast.Relationship{Kind: ast.RelReferences, Target: target}
	rel.NodeSpan = target.Span()
	return rel
}

// chainNode is the feature chain a relationship target is written as (`a.b`),
// or nil when it is a name.
func chainNode(target ast.Node) *ast.FeatureChainExpr {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	chain, _ := target.(*ast.FeatureChainExpr)
	return chain
}

// recordedRelationship is the recorded fact behind the relationship object
// rel, when its owner is a record that carries one at rel's ordinal.
func recordedRelationship(rel *symbols.Symbol) (symbols.RelationshipFacts, bool) {
	owner := rel.Implicit.Owner
	if !owner.Recorded() || rel.Implicit.Ordinal >= len(owner.Facts.Relationships) {
		return symbols.RelationshipFacts{}, false
	}
	return owner.Facts.Relationships[rel.Implicit.Ordinal], true
}

// chainTargetFeature is the feature the chain target of the relationship object
// rel denotes as a whole: the implicit chaining feature of KerML 8.3.3.3.9,
// which the relationship targets and owns, and whose chainingFeature are the
// features the chain is written as. Synthesized once per relationship and
// owned by the element the relationship is written on; nil for a relationship
// targeting a name or a chain that does not resolve.
func (m *Model) chainTargetFeature(rel *symbols.Symbol) *symbols.Symbol {
	if m == nil || rel == nil || rel.Implicit == nil {
		return nil
	}
	var (
		node  *ast.FeatureChainExpr
		facts symbols.RelationshipFacts
	)
	if rel.Implicit.Node != nil {
		node = chainNode(rel.Implicit.Node.Target)
	}
	if node == nil {
		var ok bool
		if facts, ok = recordedRelationship(rel); !ok || !facts.Chain {
			return nil
		}
	}
	owner := rel.Implicit.Owner
	defer m.own(owner).LeaveDoc()
	if cached, ok := m.chainTargets[rel]; ok {
		return cached
	}
	// A chain that resolves to nothing denotes no feature: the relationship's
	// target side stays underived, as it does for an unresolved name.
	if len(m.chainPath(owner, rel.Implicit.Kind, node, facts)) == 0 {
		return nil
	}
	scope := owner.Scope
	if scope == nil {
		// A leaf declares no scope of its own; its chain feature still needs
		// one to be owned through.
		scope = symbols.NewScope(owner.OwnerScope, nil)
		scope.SetOwner(owner)
	}
	feature := &symbols.Symbol{
		Kind:       symbols.SymbolReferenceUsage,
		DocName:    owner.DocName,
		DeclSpan:   rel.DeclSpan,
		Visibility: ast.VisibilityDefault,
		OwnerScope: scope,
		Chain:      &symbols.ChainingFeature{Relationship: rel, Node: node},
	}
	if node != nil {
		feature.Decl = node
		feature.DeclSpan = node.Span()
	}
	journal(m, m.chainTargets, rel, owner.Decl)
	m.chainTargets[rel] = feature
	return feature
}

// ChainTargetPath is the features a relationship's chain target is written as,
// outermost first, as resolved for a relationship of kind owned by sym: an end's
// attachment resolves where its connector is declared, any other target where
// sym is. Nil when target is a name or a feature of the chain resolves to nothing.
func (m *Model) ChainTargetPath(sym *symbols.Symbol, kind ast.RelationshipKind, target ast.Node) []*symbols.Symbol {
	chain := chainNode(target)
	if chain == nil {
		return nil
	}
	if m.resolver == nil {
		return nil
	}
	_, isEnd := sym.Decl.(*ast.ConnectorEnd)
	switch {
	case kind == ast.RelRedefines:
		// A redefinition's chain starts at a feature the owner inherits, not
		// at a member of its own that shadows the name (KerML 8.3.3.3.6).
		return chainPath(chain, func(prefix ast.Node) (*symbols.Symbol, bool) {
			return m.resolver.ResolveRedefinitionTarget(sym.OwnerScope, sym.Decl, prefix)
		})
	case isEnd && kind.ReferenceSubsets():
		return m.attachmentPath(referenceScope(sym), chain)
	}
	return m.attachmentPath(sym.OwnerScope, chain)
}

// chainPath is the resolved path of a relationship's chain target, from the
// tree (node) or from the record (facts); nil unless every feature resolves.
func (m *Model) chainPath(owner *symbols.Symbol, kind ast.RelationshipKind, node *ast.FeatureChainExpr, facts symbols.RelationshipFacts) []*symbols.Symbol {
	if node != nil {
		return m.ChainTargetPath(owner, kind, node)
	}
	resolved := m.recordedSequence(facts.Path)
	if len(facts.Path) == 0 || len(resolved) != len(facts.Path) {
		return nil
	}
	return resolved
}

// chainingFeaturesOf is Feature::chainingFeature of sym: for the feature a
// relationship's chain target (`s.y`) denotes as a whole, the features the
// chain is written as, in order; empty for any other feature.
func (m *Model) chainingFeaturesOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Chain == nil {
		return nil
	}
	rel := sym.Chain.Relationship
	var facts symbols.RelationshipFacts
	if sym.Chain.Node == nil {
		var ok bool
		if facts, ok = recordedRelationship(rel); !ok {
			return nil
		}
	}
	return m.chainPath(rel.Implicit.Owner, rel.Implicit.Kind, sym.Chain.Node, facts)
}
