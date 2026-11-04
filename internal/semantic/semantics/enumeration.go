package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kermlReflectivePackage is the library package holding the reflective KerML model.
const kermlReflectivePackage = "KerML"

// EnumerationOwning returns the enumeration sym is a literal of, or nil when sym
// is no enumeration literal: an enumeration usage declared in an enumeration
// definition's body (SysML v2 §7.6.4), or a member feature of a DataType the
// reflective KerML model maps a MOF enumeration into (KerML 1.1 §9.2.17).
func EnumerationOwning(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	if enum := enumeratedValueOwning(sym); enum != nil {
		return enum
	}
	if owner := sym.Owner(); sym.Kind == symbols.SymbolAttributeUsage && IsReflectiveEnumeration(owner) {
		return owner
	}
	return nil
}

// enumeratedValueOwning returns the enumeration definition sym is an enumerated
// value of, or nil when sym is not one.
func enumeratedValueOwning(sym *symbols.Symbol) *symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageEnumeration {
		return nil
	}
	return EnumerationDefinitionOwning(sym)
}

// EnumerationDefinitionOwning returns the enumeration definition whose body
// declares sym, whatever sym is, or nil when its owner is not one.
func EnumerationDefinitionOwning(sym *symbols.Symbol) *symbols.Symbol {
	owner := sym.Owner()
	if owner == nil || owner.Kind != symbols.SymbolEnumerationDef {
		return nil
	}
	return owner
}

// IsEnumeration reports whether sym is an enumeration: an enumeration definition,
// or a DataType of the reflective KerML model, whose literals are its only values.
func IsEnumeration(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolEnumerationDef || IsReflectiveEnumeration(sym))
}

// IsReflectiveEnumeration reports whether sym is a DataType of the library's reflective
// KerML model, into which KerML 1.1 §9.2.17 maps only MOF enumerations.
func IsReflectiveEnumeration(sym *symbols.Symbol) bool {
	if sym == nil || sym.Kind != symbols.SymbolAttributeDef {
		return false
	}
	root := sym
	for root.Owner() != nil {
		root = root.Owner()
	}
	return root != sym && root.Name == kermlReflectivePackage && symbols.IsLibraryPackage(root)
}

// LiteralValue returns the value expression a literal declares, or nil when it
// declares none and is identified by itself: a literal of an enumeration
// specializing a scalar type — `enum def GradePoints :> Real { A = 4.0; }` — is
// a value of that type.
func LiteralValue(sym *symbols.Symbol) ast.Node {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	return usage.Value
}

// EnumeratedValuesOf returns every literal an enumeration definition has, the unnamed
// ones (`enum def Size :> Real { = 60.0; }`) included, its own and those inherited from
// the enumerations it specializes: together they are its only instances.
func (m *Model) EnumeratedValuesOf(sym *symbols.Symbol) []*symbols.Symbol {
	if !IsEnumeration(sym) || sym.Scope == nil {
		return nil
	}
	out := m.LiteralsOf(sym)
	out = appendAnonymousLiterals(out, sym.Scope)
	for _, src := range m.MemberSources(sym) {
		out = appendAnonymousLiterals(out, src.Scope)
	}
	return out
}

func appendAnonymousLiterals(out []*symbols.Symbol, scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return out
	}
	for _, member := range scope.AnonymousMembers() {
		if EnumerationOwning(member) != nil {
			out = append(out, member)
		}
	}
	return out
}

// LiteralsOf returns the named literals an enumeration definition declares, including
// the ones it inherits from an enumeration it specializes.
func (m *Model) LiteralsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if !IsEnumeration(sym) {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range m.MembersOf(sym) {
		if EnumerationOwning(member) != nil {
			out = append(out, member)
		}
	}
	return out
}
