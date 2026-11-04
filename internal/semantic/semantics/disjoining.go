package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DisjoiningTypes returns the resolved targets of sym's `disjoint from` clauses:
// the types asserted to share no instances with sym (KerML 1.1 §7.3.2.5,
// §8.3.3.1.4 Disjoining). Memoized, in declaration order, without repetition.
func (m *Model) DisjoiningTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.composedOperands(sym, ast.RelDisjoint)
}

// Disjoint reports whether a and b provably share no instances: a type on a's
// specialization chain (a itself included) is declared disjoint from a type b
// lies within, or the reverse (KerML 1.1 §7.3.2.5: types related by disjoining
// do not share instances; a subtype's instances are its supertypes', §7.3.2.4),
// or one of them, or a type it specializes, is the union of types each disjoint
// from the other (§7.3.2.3: a union's instances are exactly its unioning types').
// Mere unrelatedness is not disjointness: two types with no disjoining between
// their chains may still share instances.
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

// within reports whether every instance of b is an instance of d: b conforms to
// d, or d is a union and b lies within one of its unioning types (KerML 1.1
// §7.3.2.3). seen guards a unioning cycle.
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

// unionDisjoint reports whether a, or a type it specializes, is declared as the
// union of types that are all disjoint from b: a union's instances are exactly
// its unioning types' (KerML 1.1 §7.3.2.3), and a subtype's are a subset of its
// supertype's. unioning guards a cycle.
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
