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
	}
	if m.computingRedefinedFeatures == 0 {
		journal(m, m.readOnly, sym, sym.Decl)
		m.readOnly[sym] = ro
	}
	return ro
}

// declaredDerived reports whether sym's own declaration says `derived`.
func declaredDerived(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.IsDerived
	case *ast.CrossFeatureMember:
		return d.IsDerived
	}
	return false
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

// constantDeclaration is the feature sym is constant by — itself, or one it
// subsets or redefines — or nil. A feature declared variable is not constant by
// inheritance; seen guards against cyclic subsetting.
func (m *Model) constantDeclaration(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) *symbols.Symbol {
	traits, ok := featureTraitsOf(sym)
	if !ok {
		return nil
	}
	if traits.IsConstant {
		return sym
	}
	if traits.IsVariable || seen[sym] {
		return nil
	}
	seen[sym] = true
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
			continue
		}
		next := m.conformanceTarget(sym, rel)
		if next == nil || next == sym {
			continue
		}
		if found := m.constantDeclaration(next, seen); found != nil {
			return found
		}
	}
	return nil
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
		return fmt.Sprintf("%s is constant%s: its value does not change over the lifetime of its featuring occurrence", text, by)
	case ReadOnlyDerived:
		return fmt.Sprintf("%s is derived%s: its values are determined by the model, not written", text, by)
	}
	return ""
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
