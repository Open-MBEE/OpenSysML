package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Feature conformance (KerML 8.3.3.3): a subsetting feature restricts the
// subsetted one and may not widen it; a redefinition replaces the redefined
// feature in the owning type, so its direction must agree too.

// ConformanceViolationKind names the conformance rule a relationship breaks.
type ConformanceViolationKind int

const (
	// ViolationDirection: a redefining feature's direction differs from the
	// direction the redefined feature has in the owning type.
	ViolationDirection ConformanceViolationKind = iota
	// ViolationUniqueness: a nonunique feature subsets or redefines a unique one.
	ViolationUniqueness
	// ViolationConstancy: a variable feature subsets or redefines a constant one.
	ViolationConstancy
)

// ConformanceViolation is one broken conformance rule: the relationship that
// breaks it, and the reference to report it at.
type ConformanceViolation struct {
	Kind ConformanceViolationKind
	// Feature is the subsetting or redefining feature.
	Feature *symbols.Symbol
	// Target is the subsetted or redefined feature.
	Target *symbols.Symbol
	// Ref is the node the diagnostic is reported at: the target reference of an
	// explicit clause, or the declaration itself for an implicit redefinition.
	Ref ast.Node
}

// ConformanceViolations returns the feature-conformance rules the declaration of
// sym breaks against the features it subsets and redefines, explicitly or as a
// parameter of its owning behavior.
func (m *Model) ConformanceViolations(sym *symbols.Symbol) []ConformanceViolation {
	traits, ok := featureTraitsOf(sym)
	if !ok {
		return nil
	}
	var out []ConformanceViolation
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		target := m.conformanceTarget(sym, rel)
		if target == nil || target == sym {
			continue
		}
		ref := ast.Node(rel.Target)
		if rel.Kind == ast.RelRedefines {
			out = append(out, m.directionViolations(sym, traits, target, ref)...)
		}
		out = append(out, m.restrictionViolations(sym, traits, target, ref)...)
	}
	// A parameter redefines the parameter at its position implicitly, and the
	// direction it declares must be the redefined one's (KerML 7.4.7.2).
	for _, target := range m.implicitParameterCounterparts(sym) {
		out = append(out, m.directionViolations(sym, traits, target, sym.Decl)...)
	}
	return out
}

// conformanceTarget resolves the feature a subsetting or redefinition names. A
// redefinition names what the owner inherits, which RedefinedFeatures already
// resolves; a subsetting is resolved like any other reference.
func (m *Model) conformanceTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if rel.Kind == ast.RelRedefines {
		return m.redefinitionTarget(sym, rel.Target)
	}
	target := rel.Target
	if ref, ok := target.(*ast.FeatureReference); ok {
		target = ref.Name
	}
	found, ok := m.resolver.ResolveTarget(sym.OwnerScope, target)
	if !ok || found == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(found); aliasOK {
		return resolved
	}
	return found
}

// directionViolations reports a redefinition whose declared direction differs
// from the redefined feature's. An undeclared direction takes the redefined
// one's, and a redefined `inout` admits any direction.
func (m *Model) directionViolations(sym *symbols.Symbol, traits featureTraits,
	target *symbols.Symbol, ref ast.Node) []ConformanceViolation {
	if traits.Direction == ast.DirNone {
		return nil
	}
	targetDir := m.directionThrough(owningTypeOf(sym), target)
	if targetDir == ast.DirNone || targetDir == ast.DirInOut || targetDir == traits.Direction {
		return nil
	}
	return []ConformanceViolation{{
		Kind: ViolationDirection, Feature: sym, Target: target, Ref: ref,
	}}
}

// restrictionViolations reports the uniqueness and constancy rules: a subsetting
// or redefining feature may not be nonunique where the target is unique, nor
// variable where the target is constant (KerML 8.3.3.3).
func (m *Model) restrictionViolations(sym *symbols.Symbol, traits featureTraits,
	target *symbols.Symbol, ref ast.Node) []ConformanceViolation {
	targetTraits, ok := featureTraitsOf(target)
	if !ok {
		return nil
	}
	var out []ConformanceViolation
	if traits.IsNonunique && m.IsUnique(target) {
		out = append(out, ConformanceViolation{
			Kind: ViolationUniqueness, Feature: sym, Target: target, Ref: ref,
		})
	}
	if traits.IsVariable && m.isConstantFeature(target, targetTraits, map[*symbols.Symbol]bool{}) {
		out = append(out, ConformanceViolation{
			Kind: ViolationConstancy, Feature: sym, Target: target, Ref: ref,
		})
	}
	return out
}

// isConstantFeature reports whether target is constant, declared so or through a
// feature it subsets or redefines: constancy is inherited by restriction.
// seen guards against cyclic subsetting.
func (m *Model) isConstantFeature(target *symbols.Symbol, traits featureTraits,
	seen map[*symbols.Symbol]bool) bool {
	if traits.IsConstant {
		return true
	}
	if traits.IsVariable || seen[target] {
		return false
	}
	seen[target] = true
	for _, rel := range RelationshipsOf(target) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		next := m.conformanceTarget(target, rel)
		if next == nil || next == target {
			continue
		}
		nextTraits, ok := featureTraitsOf(next)
		if ok && m.isConstantFeature(next, nextTraits, seen) {
			return true
		}
	}
	return false
}

// directionThrough returns the direction feature has as seen through owner:
// reversed when owner reaches the feature's owning type by a conjugation
// (SysML v2 7.12.2).
func (m *Model) directionThrough(owner, feature *symbols.Symbol) ast.FeatureDirection {
	traits, ok := featureTraitsOf(feature)
	if !ok {
		return ast.DirNone
	}
	dir := traits.Direction
	source := owningTypeOf(feature)
	if owner == nil || source == nil {
		return dir
	}
	for _, sup := range m.conjugatedSupertypes(owner) {
		if sup.sym == source && sup.conjugated {
			return ConjugateDirection(dir)
		}
	}
	return dir
}

// owningTypeOf returns the type declaring sym, or nil at the top level.
func owningTypeOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// featureTraits are the declared properties conformance compares, read off a
// usage or an inline cross feature alike.
type featureTraits struct {
	Direction   ast.FeatureDirection
	IsNonunique bool
	IsVariable  bool
	IsConstant  bool
}

// featureTraitsOf returns the traits sym's declaration states, if it is a feature.
func featureTraitsOf(sym *symbols.Symbol) (featureTraits, bool) {
	if sym == nil {
		return featureTraits{}, false
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return featureTraits{Direction: d.Direction, IsNonunique: d.IsNonunique,
			IsVariable: d.IsVariable, IsConstant: d.IsConstant}, true
	case *ast.CrossFeatureMember:
		return featureTraits{Direction: d.Direction, IsNonunique: d.IsNonunique,
			IsVariable: d.IsVariable, IsConstant: d.IsConstant}, true
	}
	return featureTraits{}, false
}
