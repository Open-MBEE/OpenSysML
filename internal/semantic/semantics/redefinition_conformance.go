package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	written := map[*symbols.Symbol]bool{}
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
		written[target] = true
		ref := ast.Node(rel.Target)
		if rel.Kind == ast.RelRedefines {
			out = append(out, m.directionViolations(sym, traits, target, ref)...)
		}
		out = append(out, m.restrictionViolations(sym, traits, target, ref)...)
	}
	// A subsetting the feature has implicitly — a composite action in an
	// action one of its `subactions` — is a Subsetting all the same, and the
	// uniqueness and constancy conformance constraints hold of it as of a
	// written one. Nothing is written to point at, so the declaration is.
	for _, target := range m.ImplicitSubsettings(sym) {
		if target == nil || target == sym || written[target] {
			continue
		}
		out = append(out, m.restrictionViolations(sym, traits, target, sym.Decl)...)
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
	if _, ok := featureTraitsOf(target); !ok {
		return nil
	}
	var out []ConformanceViolation
	if traits.IsNonunique && m.IsUnique(target) {
		out = append(out, ConformanceViolation{
			Kind: ViolationUniqueness, Feature: sym, Target: target, Ref: ref,
		})
	}
	if traits.IsVariable && m.constantDeclaration(target, map[*symbols.Symbol]bool{}) != nil {
		out = append(out, ConformanceViolation{
			Kind: ViolationConstancy, Feature: sym, Target: target, Ref: ref,
		})
	}
	return out
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

// EffectiveDirection returns a feature's direction in the context of its owner.
func (m *Model) EffectiveDirection(owner, feature *symbols.Symbol) ast.FeatureDirection {
	if m == nil || feature == nil {
		return ast.DirNone
	}
	return m.directionThrough(owner, feature)
}

// DeclaredDirection returns the direction stated on a feature declaration.
func (m *Model) DeclaredDirection(feature *symbols.Symbol) ast.FeatureDirection {
	if m == nil {
		return ast.DirNone
	}
	traits, ok := featureTraitsOf(feature)
	if !ok {
		return ast.DirNone
	}
	return traits.Direction
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
