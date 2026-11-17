package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// FeatureValueGeneral returns the result of sym's value expression, which sym specializes when it
// declares no specialization or direction (KerML 8.3.3.3.4 checkFeatureValuationSpecialization).
func (m *Model) FeatureValueGeneral(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || m.resolver == nil || !sym.IsFeature() {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.valueGenerals[sym]; ok {
		return cached
	}
	if sym.Recorded() {
		var out *symbols.Symbol
		if !sym.Facts.ValueGeneral.IsZero() {
			out = m.recordedElement(sym.Facts.ValueGeneral)
		}
		journal(m, m.valueGenerals, sym, sym.Decl)
		m.valueGenerals[sym] = out
		return out
	}
	// A subject, actor or stakeholder is directed without writing so.
	if dir, _ := ReflectiveDirection(sym); dir != ast.DirNone {
		return nil
	}
	m.resolver.Enter()
	out := m.resolver.FeatureValueGeneral(sym)
	if out == sym {
		out = nil
	}
	if m.resolver.Leave() {
		journal(m, m.valueGenerals, sym, sym.Decl)
		m.valueGenerals[sym] = out
	}
	return out
}

// generalsOutsideTyping returns the generals sym's declaration does not state, its kind's and its
// feature value's: its declared types, which the value is validated against (KerML 8.4.4.11), pass over them.
func (m *Model) generalsOutsideTyping(sym *symbols.Symbol) []*symbols.Symbol {
	bases := m.ImplicitGenerals(sym)
	general := m.FeatureValueGeneral(sym)
	if general == nil {
		return bases
	}
	return append(bases[:len(bases):len(bases)], general)
}
