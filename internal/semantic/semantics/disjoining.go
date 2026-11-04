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
// conforms to, or the reverse (KerML 1.1 §7.3.2.5: types related by disjoining
// do not share instances; a subtype's instances are its supertypes', §7.3.2.4),
// or one of them is the union of types each disjoint from the other (§7.3.2.3).
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
// disjoint from a type b conforms to.
func (m *Model) declaredDisjoint(a, b *symbols.Symbol) bool {
	for _, d := range m.DisjoiningTypes(a) {
		if m.Conforms(b, d) {
			return true
		}
	}
	for _, super := range m.AllSupertypes(a) {
		for _, d := range m.DisjoiningTypes(super) {
			if m.Conforms(b, d) {
				return true
			}
		}
	}
	return false
}

// unionDisjoint reports whether a is declared as the union of types that are all
// disjoint from b: a union's instances are exactly its unioning types' (KerML
// 1.1 §7.3.2.3). unioning guards a cycle.
func (m *Model) unionDisjoint(a, b *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	unions := m.UnioningTypes(a)
	if len(unions) == 0 || unioning[a] {
		return false
	}
	if unioning == nil {
		unioning = make(map[*symbols.Symbol]bool)
	}
	unioning[a] = true
	defer delete(unioning, a)
	for _, u := range unions {
		if !m.disjoint(u, b, unioning) {
			return false
		}
	}
	return true
}
