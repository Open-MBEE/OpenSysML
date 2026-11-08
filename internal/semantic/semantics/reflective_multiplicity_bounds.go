package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// MultiplicityBounds are the bound Expressions a written MultiplicityRange owns
// (KerML 8.3.3.6 MultiplicityRange::lowerBound, upperBound), in that order: `[n]`
// states the upper bound alone, as the conversion writes it. Nil for any other
// symbol, a default multiplicity included.
func (m *Model) MultiplicityBounds(rng *symbols.Symbol) []*symbols.Symbol {
	if m == nil || !IsDeclaredMultiplicityRange(rng) || rng.OwnerScope == nil {
		return nil
	}
	if cached, ok := m.multiplicityBounds[rng]; ok {
		return cached
	}
	mult := UsageMultiplicityOf(rng.OwnerScope.Owner())
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
