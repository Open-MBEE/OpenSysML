package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// CrossSubsettings returns the `crosses` relationships sym declares, in order.
func CrossSubsettings(sym *symbols.Symbol) []*ast.Relationship {
	if sym == nil {
		return nil
	}
	var out []*ast.Relationship
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelCrosses && rel.Target != nil {
			out = append(out, rel)
		}
	}
	return out
}

// CrossedFeatureChain resolves the two features a cross subsetting chains, the
// end it starts from and the feature it reaches; ok is false for any other shape.
func (m *Model) CrossedFeatureChain(sym *symbols.Symbol, rel *ast.Relationship) (base, cross *symbols.Symbol, ok bool) {
	if m == nil || m.resolver == nil || sym == nil || rel == nil {
		return nil, nil, false
	}
	fce, isChain := rel.Target.(*ast.FeatureChainExpr)
	if !isChain || fce.Member == nil || len(fce.Member.Parts) != 1 {
		return nil, nil, false
	}
	switch fce.Operand.(type) {
	case *ast.FeatureReference, *ast.QualifiedName:
	default:
		return nil, nil, false
	}
	if found, resolved := m.resolver.ResolveTarget(sym.OwnerScope, fce.Operand); resolved {
		base = m.aliasTarget(found)
	}
	if found, resolved := m.resolver.ResolveTarget(sym.OwnerScope, fce); resolved {
		cross = m.aliasTarget(found)
	}
	return base, cross, true
}

// CrossFeature returns Feature::crossFeature: the feature sym's first cross
// subsetting reaches, else the cross feature an end declares in its body, or nil.
func (m *Model) CrossFeature(sym *symbols.Symbol) *symbols.Symbol {
	crosses := CrossSubsettings(sym)
	if len(crosses) == 0 {
		return m.OwnedCrossFeature(sym)
	}
	_, cross, _ := m.CrossedFeatureChain(sym, crosses[0])
	return cross
}

// OwnedCrossFeature returns the feature the end sym declares inline ahead of
// itself, else the first `member feature` in its body: its cross feature when
// it crosses nothing explicitly.
func (m *Model) OwnedCrossFeature(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Scope == nil || !declaresEnd(sym) || ownerSymbol(sym) == nil {
		return nil
	}
	if u, ok := sym.Decl.(*ast.Usage); ok && u.CrossFeature != nil {
		return memberSymbol(sym.Scope, u.CrossFeature)
	}
	for _, member := range declMembers(sym) {
		wrapper, ok := member.(*ast.Membership)
		if !ok || !wrapper.IsTypeFeature {
			continue
		}
		if usage, isUsage := wrapper.Member.(*ast.Usage); isUsage {
			return memberSymbol(sym.Scope, usage)
		}
	}
	return nil
}

// owningEndOf returns the end feature whose owned cross feature sym is, or nil.
func (m *Model) owningEndOf(sym *symbols.Symbol) *symbols.Symbol {
	end := ownerSymbol(sym)
	if end == nil || m.OwnedCrossFeature(end) != sym {
		return nil
	}
	return end
}

// implicitCrossFeatureGenerals returns what an owned cross feature implicitly
// specializes: its end's declared types, then the cross features of the ends its
// end redefines (KerML 1.1 §8.3.3.3, checkFeatureOwnedCrossFeatureSpecialization).
func (m *Model) implicitCrossFeatureGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	end := m.owningEndOf(sym)
	if end == nil {
		return nil
	}
	out := append([]*symbols.Symbol(nil), m.DeclaredFeatureTypes(end)...)
	redefined := append([]*symbols.Symbol(nil), m.RedefinedFeatures(end)...)
	for _, general := range append(redefined, m.implicitEndRedefinitions(end)...) {
		if !declaresEnd(general) {
			continue
		}
		if cross := m.CrossFeature(general); cross != nil && cross != sym {
			out = append(out, cross)
		}
	}
	return out
}

// EndFeatures returns the end features of the type sym, its own in declaration
// order followed by those it inherits and does not redefine.
func (m *Model) EndFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	return m.endsOf(sym)
}

func (m *Model) aliasTarget(sym *symbols.Symbol) *symbols.Symbol {
	if m.resolver == nil {
		return sym
	}
	if resolved, ok := m.resolver.ResolveAliasTarget(sym); ok {
		return resolved
	}
	return sym
}
