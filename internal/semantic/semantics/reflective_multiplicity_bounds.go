package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// MultiplicityBounds are the bound Expressions a written MultiplicityRange owns
// (KerML 8.3.3.6 MultiplicityRange::lowerBound, upperBound), in that order: `[n]`
// states the upper bound alone, as the conversion writes it. The range is a
// feature's clause or a `multiplicity m [a..b]` declaration (KerML.xtext
// MultiplicityRange). Nil for any other symbol, a default multiplicity included.
func (m *Model) MultiplicityBounds(rng *symbols.Symbol) []*symbols.Symbol {
	if m == nil || rng == nil {
		return nil
	}
	if cached, ok := m.multiplicityBounds[rng]; ok {
		return cached
	}
	mult := declaredRangeClause(rng)
	if mult == nil {
		return nil
	}
	lower, upper := mult.Lower, mult.Upper
	if !mult.IsRange {
		lower, upper = nil, mult.Lower
	}
	var out []*symbols.Symbol
	for _, bound := range []struct {
		slot string
		node ast.Node
	}{{"lowerBound", lower}, {"upperBound", upper}} {
		if bound.node == nil {
			continue
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    rng.DocName,
			Decl:       bound.node,
			DeclSpan:   bound.node.Span(),
			Visibility: rng.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: rng, Role: symbols.ExpressionBound, Slot: bound.slot},
		})
	}
	if m.multiplicityBounds == nil {
		m.multiplicityBounds = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
	journal(m, m.multiplicityBounds, rng, rng.Decl)
	m.multiplicityBounds[rng] = out
	return out
}

// declaredRangeClause is the written clause a MultiplicityRange symbol states: the
// owning feature's for the range its clause implies, a multiplicity declaration's own.
func declaredRangeClause(rng *symbols.Symbol) *ast.Multiplicity {
	if decl, ok := rng.Decl.(*ast.MultiplicityDecl); ok {
		return decl.Range
	}
	if !IsDeclaredMultiplicityRange(rng) || rng.OwnerScope == nil {
		return nil
	}
	return UsageMultiplicityOf(rng.OwnerScope.Owner())
}
