package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DisjoiningTypes returns the resolved targets of sym's `disjoint from` clauses,
// the types asserted to share no instances with sym (KerML 1.1 §7.3.2.5, §8.3.3.1.4).
func (m *Model) DisjoiningTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.composedOperands(sym, ast.RelDisjoint)
}

// Disjoint reports whether a and b provably share no instances by a declared or
// inherited disjoining (KerML 1.1 §7.3.2.5, §7.3.2.4) or a union of such types (§7.3.2.3).
func (m *Model) Disjoint(a, b *symbols.Symbol) bool {
	return m.disjoint(a, b, nil)
}

func (m *Model) disjoint(a, b *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	if a == nil || b == nil || symbols.SameElement(a, b) {
		return false
	}
	return m.declaredDisjoint(a, b) || m.declaredDisjoint(b, a) ||
		m.unionDisjoint(a, b, unioning) || m.unionDisjoint(b, a, unioning)
}

// declaredDisjoint reports whether a, or a type it specializes, declares itself
// disjoint from a type b lies within.
func (m *Model) declaredDisjoint(a, b *symbols.Symbol) bool {
	for _, t := range m.selfAndSupertypes(a) {
		for _, d := range m.DisjoiningTypes(t) {
			if m.within(b, d, nil) {
				return true
			}
		}
	}
	return false
}

// within reports whether b conforms to d or lies within a unioning type of d
// (KerML 1.1 §7.3.2.3); seen guards unioning cycles.
func (m *Model) within(b, d *symbols.Symbol, seen map[*symbols.Symbol]bool) bool {
	if m.Conforms(b, d) {
		return true
	}
	unions := m.UnioningTypes(d)
	if len(unions) == 0 || seen[d] {
		return false
	}
	if seen == nil {
		seen = make(map[*symbols.Symbol]bool)
	}
	seen[d] = true
	defer delete(seen, d)
	for _, u := range unions {
		if m.within(b, u, seen) {
			return true
		}
	}
	return false
}

// unionDisjoint reports whether a, or a type it specializes, is the union of types
// all disjoint from b (KerML 1.1 §7.3.2.3); unioning guards unioning cycles.
func (m *Model) unionDisjoint(a, b *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	for _, t := range m.selfAndSupertypes(a) {
		unions := m.UnioningTypes(t)
		if len(unions) == 0 || unioning[t] {
			continue
		}
		if unioning == nil {
			unioning = make(map[*symbols.Symbol]bool)
		}
		unioning[t] = true
		all := true
		for _, u := range unions {
			if !m.disjoint(u, b, unioning) {
				all = false
				break
			}
		}
		delete(unioning, t)
		if all {
			return true
		}
	}
	return false
}

func (m *Model) selfAndSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	return append([]*symbols.Symbol{sym}, m.AllSupertypes(sym)...)
}
