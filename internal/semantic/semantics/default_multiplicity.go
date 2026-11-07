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
	if m == nil || sym == nil || sym.Implicit != nil || sym.Chain != nil || sym.Expr != nil || m.isKerMLDoc(sym) {
		return false
	}
	if IsIndividualDefinition(sym) {
		// An `individual` definition's prefix owns an empty Multiplicity (SysML.xtext
		// OccurrenceDefinitionPrefix, EmptyMultiplicityMember), first among its relationships.
		return !m.declaresMultiplicity(sym)
	}
	if sym.Kind == symbols.SymbolConnectorEnd {
		// Every connector end without a written range is held to [1..1], positional
		// (`connect x to y`) or named (SysML v2 §7.13.2); a flow's ends (FlowEnd) take none.
		if end, ok := sym.Decl.(*ast.ConnectorEnd); ok && end.Multiplicity != nil {
			return false
		}
		if owner := sym.OwnerScope.OwningElement(); owner != nil {
			if usage, ok := owner.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageFlow {
				return false
			}
		}
		return !m.declaresMultiplicity(sym)
	}
	if !sym.DeclaresUsage() || m.bodyDeclaredBy(sym) != nil {
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
// (DeclaredMultiplicityMember) or the implicit one it is parsed with (DefaultMultiplicityMember).
func (m *Model) MultiplicityMember(sym *symbols.Symbol) *symbols.Symbol {
	if member := m.DeclaredMultiplicityMember(sym); member != nil {
		return member
	}
	return m.DefaultMultiplicityMember(sym)
}

// IsImpliedMultiplicity reports whether sym is a declaration's declared range or default
// Multiplicity (MultiplicityMember).
func IsImpliedMultiplicity(sym *symbols.Symbol) bool {
	decl, ok := symbols.ImpliedDeclOf(sym)
	if !ok {
		return false
	}
	switch decl.Form {
	case symbols.ImpliedDefaultMultiplicity, symbols.ImpliedMultiplicityRange:
		return true
	}
	return false
}

// IsIndividualDefinition reports whether sym is a definition declared `individual`
// (an OccurrenceDefinition with isIndividual, SysML v2 §7.9.2).
func IsIndividualDefinition(sym *symbols.Symbol) bool {
	if sym == nil || !sym.DeclaresDefinition() {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModIndividual)
	}
	d, ok := sym.Decl.(*ast.Definition)
	return ok && d.IsIndividual
}

// defaultMultiplicityFirst reports whether sym's default Multiplicity is owned first
// (an `individual` definition's prefix) rather than last.
func defaultMultiplicityFirst(sym *symbols.Symbol) bool {
	return IsIndividualDefinition(sym)
}
