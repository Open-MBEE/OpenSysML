package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ReflectiveFeatureTypes reports Feature::type (KerML 8.3.3.3.4 deriveFeatureType) as
// the reflective layer answers it: false when an input relationship is unresolved.
func (m *Model) ReflectiveFeatureTypes(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if typ, annotation := m.annotationMetadataType(sym); annotation {
		if typ == nil {
			return nil, false
		}
		return []*symbols.Symbol{typ}, true
	}
	if !m.FeatureTypeInputsComplete(sym) {
		return nil, false
	}
	types := m.FeatureTypeSet(sym)
	if len(types) == 0 {
		if _, hasBase := m.FeatureBaseFQN(sym); hasBase {
			return nil, false
		}
	}
	return types, true
}

// FeatureTypeInputsComplete reports whether every feature Feature::type reads
// through — sym and what it subsets, references and redefines — resolves whole.
func (m *Model) FeatureTypeInputsComplete(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	seen := make(map[*symbols.Symbol]bool)
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		if candidate == nil || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if m.SupertypesProvisional(candidate) ||
			m.HasSpecializationCycle(candidate) ||
			!m.GeneralizationsResolve(candidate) {
			return false
		}
		for _, relationship := range RelationshipsOf(candidate) {
			if relationship == nil || relationship.Kind == ast.RelTyping ||
				!GeneralizationKind(relationship.Kind) {
				continue
			}
			target := m.RelationshipTarget(candidate, relationship)
			if target == nil {
				return false
			}
			if target.IsFeature() {
				queue = append(queue, target)
			}
		}
	}
	return true
}

// GeneralizationsResolve reports whether every generalization and reference
// subsetting sym writes names a resolved target.
func (m *Model) GeneralizationsResolve(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil {
			return false
		}
		if !GeneralizationKind(rel.Kind) && !rel.Kind.ReferenceSubsets() {
			continue
		}
		if rel.Target == nil || m.RelationshipTarget(sym, rel) == nil {
			return false
		}
	}
	return true
}
