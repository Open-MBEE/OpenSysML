package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ImpliesDefaultMultiplicity reports whether sym's implicit [1..1] (SysML v2 §7.6.3,
// ImplicitlyDeclaresOne) is stated by a Multiplicity it owns: a usage that declares
// no multiplicity of its own, as the pilot's UsageParserPostProcessor adds one.
func (m *Model) ImpliesDefaultMultiplicity(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Chain != nil || sym.Expr != nil {
		return false
	}
	if !sym.DeclaresUsage() || m.isKerMLDoc(sym) || m.bodyDeclaredBy(sym) != nil {
		// What an expression body's scope holds, its parameters first, takes no default.
		return false
	}
	if IsVariant(sym) {
		// A variant is held by a VariantMembership, not a FeatureMembership: it is no
		// feature of its variation (SysML v2 §7.6.7), and the default is a feature's.
		return false
	}
	if m.declaresMultiplicity(sym) {
		return false
	}
	return m.ImplicitlyDeclaresOne(sym)
}

// DefaultMultiplicityMember is the Multiplicity stating sym's implicit [1..1], the
// member of the last of sym's OwnedMemberships; nil where ImpliesDefaultMultiplicity
// is false. It is a derived element: no text, diagram or outline shows it.
func (m *Model) DefaultMultiplicityMember(sym *symbols.Symbol) *symbols.Symbol {
	if !m.ImpliesDefaultMultiplicity(sym) {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.defaultMultiplicities[sym]; ok {
		return cached
	}
	member := impliedMultiplicity(sym, sym.DeclSpan, symbols.ImpliedDefaultMultiplicity)
	journal(m, m.defaultMultiplicities, sym, sym.Decl)
	m.defaultMultiplicities[sym] = member
	return member
}

// impliedMultiplicity is the nameless Multiplicity of form owner is parsed with,
// owned through owner's scope — or one made for a leaf, which declares none.
func impliedMultiplicity(owner *symbols.Symbol, span source.Span, form symbols.ImpliedForm) *symbols.Symbol {
	scope := owner.Scope
	if scope == nil {
		scope = symbols.NewScope(owner.OwnerScope, nil)
		scope.SetOwner(owner)
	}
	return &symbols.Symbol{
		Kind:       symbols.SymbolMultiplicity,
		DocName:    owner.DocName,
		DeclSpan:   span,
		Visibility: ast.VisibilityDefault,
		OwnerScope: scope,
		Decl:       &symbols.ImpliedDecl{Of: owner.Decl, Form: form},
	}
}

// ImpliesIndividualMultiplicity reports whether sym is an individual OccurrenceDefinition,
// which is parsed with a nested Multiplicity, the source of its implied Subsetting of
// Base::zeroOrOne (SysML v2 §8.3.9.3 checkOccurrenceDefinitionMultiplicitySpecialization).
func (m *Model) ImpliesIndividualMultiplicity(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || sym.Implicit != nil || !sym.DeclaresDefinition() || m.isKerMLDoc(sym) {
		return false
	}
	if !m.reflectiveMetaclassConforms(sym, "OccurrenceDefinition") {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModIndividual)
	}
	def, ok := sym.Decl.(*ast.Definition)
	return ok && def.IsIndividual
}

// IndividualMultiplicityMember is the Multiplicity an individual OccurrenceDefinition is
// parsed with, the member of the first of its OwnedMemberships, as the pilot orders it;
// nil where ImpliesIndividualMultiplicity is false. No text, diagram or outline shows it.
func (m *Model) IndividualMultiplicityMember(sym *symbols.Symbol) *symbols.Symbol {
	if !m.ImpliesIndividualMultiplicity(sym) {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.individualMultiplicities[sym]; ok {
		return cached
	}
	member := impliedMultiplicity(sym, sym.DeclSpan, symbols.ImpliedIndividualMultiplicity)
	journal(m, m.individualMultiplicities, sym, sym.Decl)
	m.individualMultiplicities[sym] = member
	return member
}

// ImpliesMultiplicity reports whether sym is parsed with a Multiplicity the notation
// writes no range for: a usage's implicit [1..1] or an individual OccurrenceDefinition's.
func (m *Model) ImpliesMultiplicity(sym *symbols.Symbol) bool {
	return m.ImpliesDefaultMultiplicity(sym) || m.ImpliesIndividualMultiplicity(sym)
}

// IsIndividualMultiplicity reports whether sym is the Multiplicity an individual
// OccurrenceDefinition is parsed with (IndividualMultiplicityMember).
func IsIndividualMultiplicity(sym *symbols.Symbol) bool {
	decl, ok := symbols.ImpliedDeclOf(sym)
	return ok && decl.Form == symbols.ImpliedIndividualMultiplicity
}

// IsDeclaredMultiplicityRange reports whether sym is the MultiplicityRange a usage's
// written range declares (DeclaredMultiplicityMember), owned at the clause.
func IsDeclaredMultiplicityRange(sym *symbols.Symbol) bool {
	decl, ok := symbols.ImpliedDeclOf(sym)
	return ok && decl.Form == symbols.ImpliedMultiplicityRange
}

// IsDefaultMultiplicity reports whether sym is the Multiplicity a usage's implicit
// [1..1] is stated by (DefaultMultiplicityMember).
func IsDefaultMultiplicity(sym *symbols.Symbol) bool {
	decl, ok := symbols.ImpliedDeclOf(sym)
	return ok && decl.Form == symbols.ImpliedDefaultMultiplicity
}

// declaresMultiplicity reports whether sym writes a multiplicity of its own, a
// range it cannot evaluate included: the record's bounds, or the declaration's clause.
func (m *Model) declaresMultiplicity(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Multiplicity != nil
	}
	if UsageMultiplicityOf(sym) != nil {
		return true
	}
	_, declared := m.MultiplicityOf(sym)
	return declared
}

// DeclaredMultiplicityMember is the nameless MultiplicityRange a usage's written range
// (`[4]`) declares, owned at the clause as the conversion writes it; nil without a range.
func (m *Model) DeclaredMultiplicityMember(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Chain != nil || sym.Expr != nil || !sym.DeclaresUsage() {
		return nil
	}
	mult := UsageMultiplicityOf(sym)
	if mult == nil {
		if !sym.Recorded() || sym.Facts.Multiplicity == nil {
			return nil
		}
	}
	if cached, ok := m.declaredMultiplicities[sym]; ok {
		return cached
	}
	span := sym.DeclSpan
	if mult != nil {
		span = mult.Span()
	}
	member := impliedMultiplicity(sym, span, symbols.ImpliedMultiplicityRange)
	journal(m, m.declaredMultiplicities, sym, sym.Decl)
	m.declaredMultiplicities[sym] = member
	return member
}

// MultiplicityMember is the Multiplicity sym states: its declared range
// (DeclaredMultiplicityMember), its implicit [1..1] (DefaultMultiplicityMember) or,
// for an individual OccurrenceDefinition, the one it is parsed with (IndividualMultiplicityMember).
func (m *Model) MultiplicityMember(sym *symbols.Symbol) *symbols.Symbol {
	if member := m.DeclaredMultiplicityMember(sym); member != nil {
		return member
	}
	if member := m.DefaultMultiplicityMember(sym); member != nil {
		return member
	}
	return m.IndividualMultiplicityMember(sym)
}

// IsImpliedMultiplicity reports whether sym is a declaration's declared range, default
// or individual Multiplicity (MultiplicityMember).
func IsImpliedMultiplicity(sym *symbols.Symbol) bool {
	decl, ok := symbols.ImpliedDeclOf(sym)
	if !ok {
		return false
	}
	switch decl.Form {
	case symbols.ImpliedDefaultMultiplicity, symbols.ImpliedMultiplicityRange, symbols.ImpliedIndividualMultiplicity:
		return true
	}
	return false
}
