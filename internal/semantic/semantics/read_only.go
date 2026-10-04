package semantics

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ReadOnlyKind is why a feature's values may not be written once its featuring
// occurrence is initialized.
type ReadOnlyKind uint8

const (
	// Writable features may be written by a behavior.
	Writable ReadOnlyKind = iota
	// ReadOnlyConstant features keep one value over the lifetime of their featuring
	// occurrence (KerML Feature::isConstant).
	ReadOnlyConstant
	// ReadOnlyDerived features have values the model determines from other
	// features (KerML Feature::isDerived).
	ReadOnlyDerived
)

// ReadOnly is a feature's write restriction and the feature declaring it, which is
// the feature itself or one it redefines or subsets.
type ReadOnly struct {
	Kind     ReadOnlyKind
	Declared *symbols.Symbol
	// ImpliedByEnd marks Declared as constant as an end that may vary in time,
	// not by `constant` (SysML 2.0 §8.4.2.2).
	ImpliedByEnd bool
}

// String names the restriction as its keyword.
func (k ReadOnlyKind) String() string {
	switch k {
	case ReadOnlyConstant:
		return "constant"
	case ReadOnlyDerived:
		return "derived"
	}
	return "writable"
}

// FeatureReadOnly reports whether sym is a feature no behavior may write: declared
// `derived`, or redefining one that is, since a redefining feature has the same
// values as the feature it redefines (KerML 1.0 §7.3.4.5); else declared `constant`, or
// subsetting or redefining a feature that is (KerML 1.0 §8.3.3.3,
// validateSubsettingConstantConformance).
func (m *Model) FeatureReadOnly(sym *symbols.Symbol) ReadOnly {
	if m == nil || sym == nil {
		return ReadOnly{}
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.readOnly[sym]; ok {
		return cached
	}
	ro := ReadOnly{}
	if declared := m.derivedDeclaration(sym, map[*symbols.Symbol]bool{sym: true}); declared != nil {
		ro = ReadOnly{Kind: ReadOnlyDerived, Declared: declared}
	} else if declared := m.constantDeclaration(sym, map[*symbols.Symbol]bool{}); declared != nil {
		ro = ReadOnly{Kind: ReadOnlyConstant, Declared: declared}
		if mods, ok := featureModifiers(declared); ok {
			ro.ImpliedByEnd = !mods.Has(symbols.ModConstant)
		}
	}
	if m.computingRedefinedFeatures == 0 {
		journal(m, m.readOnly, sym, sym.Decl)
		m.readOnly[sym] = ro
	}
	return ro
}

// featureModifiers are the modifiers sym's declaration, or its interface record,
// states; ok is false for a symbol that declares no feature.
func featureModifiers(sym *symbols.Symbol) (mods symbols.Modifiers, ok bool) {
	set := func(on bool, mod symbols.Modifiers) {
		if on {
			mods |= mod
		}
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		set(d.IsConstant, symbols.ModConstant)
		set(d.IsVariable, symbols.ModVariable)
		set(d.IsDerived, symbols.ModDerived)
		return mods, true
	case *ast.CrossFeatureMember:
		set(d.IsConstant, symbols.ModConstant)
		set(d.IsVariable, symbols.ModVariable)
		set(d.IsDerived, symbols.ModDerived)
		return mods, true
	}
	if sym.Recorded() && (sym.Facts.Node == symbols.NodeUsage || sym.Facts.Node == symbols.NodeCrossFeature) {
		return sym.Facts.Modifiers, true
	}
	return 0, false
}

// declaredDerived reports whether sym's own declaration says `derived`.
func declaredDerived(sym *symbols.Symbol) bool {
	mods, ok := featureModifiers(sym)
	return ok && mods.Has(symbols.ModDerived)
}

// derivedDeclaration is the feature along sym's redefinition chain declared
// `derived`, or nil; path holds the features being visited.
func (m *Model) derivedDeclaration(sym *symbols.Symbol, path map[*symbols.Symbol]bool) *symbols.Symbol {
	if declaredDerived(sym) {
		return sym
	}
	for _, general := range m.directRedefinedFeatures(sym) {
		if general == nil || path[general] {
			continue
		}
		path[general] = true
		found := m.derivedDeclaration(general, path)
		delete(path, general)
		if found != nil {
			return found
		}
	}
	return nil
}

// constantDeclaration is the feature sym is constant by — itself, or one it subsets
// or (implicitly) redefines — or nil; a variable feature inherits no constancy.
func (m *Model) constantDeclaration(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) *symbols.Symbol {
	mods, ok := featureModifiers(sym)
	if !ok {
		return nil
	}
	if mods.Has(symbols.ModConstant) || m.implicitlyConstantEnd(sym) {
		return sym
	}
	if mods.Has(symbols.ModVariable) || seen[sym] {
		return nil
	}
	seen[sym] = true
	for _, next := range m.constancyGenerals(sym) {
		if found := m.constantDeclaration(next, seen); found != nil {
			return found
		}
	}
	return nil
}

// constancyGenerals are the features sym's constancy is inherited from: those it
// subsets, as declared or recorded, and those it explicitly or implicitly redefines.
func (m *Model) constancyGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	if sym.Recorded() {
		out = m.recordedElements(sym, sym.RecordedRelationships(ast.RelSubsets))
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubsets {
			continue
		}
		if next := m.conformanceTarget(sym, rel); next != nil && next != sym {
			out = append(out, next)
		}
	}
	return append(out, m.directRedefinedFeatures(sym)...)
}

// ReadOnlyViolation words a write to the read-only feature text names, for the
// static and runtime checks alike: why it is read-only, and the feature it is
// so by when that is another one it redefines or subsets.
func ReadOnlyViolation(text string, sym *symbols.Symbol, ro ReadOnly) string {
	by := ""
	if ro.Declared != nil && ro.Declared != sym {
		by = " by " + ownedFeatureText(ro.Declared) + ", which it redefines or subsets"
	}
	switch ro.Kind {
	case ReadOnlyConstant:
		if ro.ImpliedByEnd {
			if ro.Declared == nil || ro.Declared == sym {
				return fmt.Sprintf("%s is constant as an end feature: its value does not change over the lifetime of its featuring occurrence", text)
			}
			return fmt.Sprintf("%s is constant by end feature %s, which it redefines or subsets: its value does not change over the lifetime of its featuring occurrence", text, ownedFeatureText(ro.Declared))
		}
		return fmt.Sprintf("%s is constant%s: its value does not change over the lifetime of its featuring occurrence", text, by)
	case ReadOnlyDerived:
		return fmt.Sprintf("%s is derived%s: its values are determined by the model, not written", text, by)
	}
	return ""
}

// FeatureIsConstant derives Feature::isConstant: declared `constant`, or implied for an end that may vary in time.
func (m *Model) FeatureIsConstant(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if mods, ok := featureModifiers(sym); ok && mods.Has(symbols.ModConstant) {
		return true
	}
	return m.implicitlyConstantEnd(sym)
}

// implicitlyConstantEnd reports a SysML end usage that may vary in time, which is constant though `constant` is not notated (KerML 1.0 §8.3.3.3.4; SysML 2.0 §8.4.2.2).
func (m *Model) implicitlyConstantEnd(sym *symbols.Symbol) bool {
	if sym == nil || m.isKerMLDoc(sym) {
		return false
	}
	if d, ok := sym.Decl.(*ast.Usage); ok {
		return d.IsEnd && m.UsageMayTimeVary(sym)
	}
	return sym.Recorded() && sym.Facts.Modifiers.Has(symbols.ModEnd) && m.UsageMayTimeVary(sym)
}

// ownedFeatureText names a feature by its owner and its own name (`Base::x`).
func ownedFeatureText(sym *symbols.Symbol) string {
	if sym.OwnerScope != nil {
		if owner := sym.OwnerScope.Owner(); owner != nil && owner.Name != "" {
			return owner.Name + "::" + sym.Name
		}
	}
	return sym.Name
}
