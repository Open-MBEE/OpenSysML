package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DeclaresVariation reports whether sym is declared with the `variation`
// modifier, which makes it a variation point: an abstract classifier of the
// variants declared for it (SysML v2 §7.20, VariantMembership).
func DeclaresVariation(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.IsVariation
	case *ast.Usage:
		return d.IsVariation
	}
	return false
}

// IsVariation reports whether sym is a variation: declared `variation`, or an
// enumeration definition, whose isVariation is always true (SysML v2 §8.3.9).
func IsVariation(sym *symbols.Symbol) bool {
	return DeclaresVariation(sym) || (sym != nil && sym.Kind == symbols.SymbolEnumerationDef)
}

// DeclaresVariant reports whether sym is declared with the `variant` keyword,
// which makes it one of the choices of the variation that owns it.
func DeclaresVariant(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsVariant
}

// IsVariant reports whether sym is a variant: declared `variant`, or an
// enumerated value, held by a variant membership (SysML v2 §7.6.4).
func IsVariant(sym *symbols.Symbol) bool {
	return DeclaresVariant(sym) || EnumerationOwning(sym) != nil
}

// VariantValue returns the value expression a variant declares, or nil when it
// declares none and stands for an object of itself.
func VariantValue(sym *symbols.Symbol) ast.Node {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	return usage.Value
}

// VariationOwning returns the variation sym is a variant of — the declaration
// owning the variant membership — or nil when sym is not a variant. An
// enumerated value is a variant of the enumeration definition owning it.
func VariationOwning(sym *symbols.Symbol) *symbols.Symbol {
	if !IsVariant(sym) || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !IsVariation(owner) {
		return nil
	}
	return owner
}

// VariationPointOwning returns the variation point sym is a variant of, or nil
// when sym is not a variant of one: unlike VariationOwning it accepts an owner
// that is a variation by specialization without restating the modifier.
func (m *Model) VariationPointOwning(sym *symbols.Symbol) *symbols.Symbol {
	if !IsVariant(sym) || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !m.IsVariationFeature(owner) {
		return nil
	}
	return owner
}

// IsVariationFeature reports whether sym is a variation point: a variation itself
// (declared, or an enumeration definition) or a usage specializing a declared one.
// A usage typed by an enumeration holds one of its values, as any attribute does.
func (m *Model) IsVariationFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if IsVariation(sym) {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if DeclaresVariation(sup) {
			return true
		}
	}
	return false
}

// VariantsOf returns the variants sym offers, in declaration order: those
// declared for it and those it inherits from the variation it specializes,
// the enumerated values of an enumeration among them. A `variant` inherited
// from a type that is not a variation point offers no choice, so it is an
// ordinary member here too.
func (m *Model) VariantsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range m.MembersOf(sym) {
		if m.VariationPointOwning(member) != nil {
			out = append(out, member)
		}
	}
	return out
}

// VariantOf returns the variant of sym named name, and whether sym offers one.
func (m *Model) VariantOf(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, variant := range m.VariantsOf(sym) {
		if variant.Name == name {
			return variant, true
		}
	}
	return nil, false
}

// SelectsVariantOf reports whether variant is a variant sym may be bound to:
// one declared for sym itself, or for a variation sym specializes — a usage
// redefining a variation selects among the variants of what it redefines.
func (m *Model) SelectsVariantOf(sym, variant *symbols.Symbol) bool {
	owning := m.VariationPointOwning(variant)
	if owning == nil || sym == nil {
		return false
	}
	if owning == sym {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if sup == owning {
			return true
		}
	}
	return false
}
