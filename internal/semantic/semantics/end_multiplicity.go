package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// EndMultiplicityIsOne reports whether end feature sym declares, or takes from a
// general, the multiplicity 1..1 (KerML 1.1 §8.3.3.3); a SysML end defaults to it.
func (m *Model) EndMultiplicityIsOne(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	w := &multiplicityWalk{m: m, seen: make(map[*symbols.Symbol]bool)}
	return w.symbolIsOne(sym)
}

// ConnectorEndMultiplicityIsOne is EndMultiplicityIsOne for the unnamed end at
// position i of connector sym (`connector c (a, b)`).
func (m *Model) ConnectorEndMultiplicityIsOne(sym *symbols.Symbol, i int) bool {
	if m == nil || sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || i < 0 || i >= len(usage.ConnectorEnds) || usage.ConnectorEnds[i] == nil {
		return false
	}
	if !m.isKerMLDoc(sym) {
		return true
	}
	w := &multiplicityWalk{m: m, seen: make(map[*symbols.Symbol]bool)}
	if target := usage.ConnectorEnds[i].AttachedTarget(); target != nil {
		if general := w.resolve(sym.OwnerScope, target); general != nil && w.symbolIsOne(general) {
			return true
		}
	}
	for _, general := range m.positionalEnds(sym, i) {
		if w.symbolIsOne(general) {
			return true
		}
	}
	return false
}

// multiplicityWalk finds a multiplicity of exactly one along the generals of a
// type, visiting each symbol once so cyclic specialization terminates.
type multiplicityWalk struct {
	m    *Model
	seen map[*symbols.Symbol]bool
}

// symbolIsOne reports whether sym's multiplicities include exactly one. A type
// declaring a multiplicity has that one alone; otherwise it has its generals'.
func (w *multiplicityWalk) symbolIsOne(sym *symbols.Symbol) bool {
	if sym == nil || w.seen[sym] {
		return false
	}
	w.seen[sym] = true
	if decl, ok := sym.Decl.(*ast.MultiplicityDecl); ok {
		return w.declIsOne(decl, sym.OwnerScope)
	}
	if mult, decl, declared := ownMultiplicity(sym); declared {
		if mult != nil {
			r, ok := w.m.multiplicityRangeIn(declScope(sym), mult)
			return ok && isExactlyOne(r)
		}
		return w.declIsOne(decl, sym.Scope)
	}
	if declaresEnd(sym) && !w.m.isKerMLDoc(sym) {
		return true
	}
	for _, general := range w.generals(sym) {
		if w.symbolIsOne(general) {
			return true
		}
	}
	return false
}

// declIsOne reports whether a `multiplicity` member is exactly one: by the
// range it writes, or by the multiplicity it subsets.
func (w *multiplicityWalk) declIsOne(decl *ast.MultiplicityDecl, scope *symbols.Scope) bool {
	if decl == nil {
		return false
	}
	if decl.Range != nil {
		r, ok := w.m.multiplicityRangeIn(scope, decl.Range)
		return ok && isExactlyOne(r)
	}
	if decl.Subsets == nil {
		return false
	}
	return w.symbolIsOne(w.resolve(scope, decl.Subsets))
}

// generals returns every type sym takes multiplicities from when it declares
// none: supertypes, what it references, crosses or chains to, positional ends.
func (w *multiplicityWalk) generals(sym *symbols.Symbol) []*symbols.Symbol {
	out := w.m.allGenerals(sym)
	if end, ok := sym.Decl.(*ast.ConnectorEnd); ok {
		out = append(out, w.resolve(enclosingScope(sym), end.AttachedTarget()))
	} else {
		for _, rel := range RelationshipsOf(sym) {
			if rel == nil || rel.Target == nil {
				continue
			}
			switch rel.Kind {
			case ast.RelReferences, ast.RelCrosses, ast.RelChains:
				out = append(out, w.resolve(sym.OwnerScope, rel.Target))
			}
		}
	}
	if declaresEnd(sym) {
		if owner := ownerSymbol(sym); owner != nil {
			if i := endIndex(owner, sym); i >= 0 {
				out = append(out, w.m.positionalEnds(owner, i)...)
			}
		}
	}
	return out
}

// resolve returns the symbol target names in scope, or nil.
func (w *multiplicityWalk) resolve(scope *symbols.Scope, target ast.Node) *symbols.Symbol {
	if w.m.resolver == nil || scope == nil || target == nil {
		return nil
	}
	found, ok := w.m.resolver.ResolveTarget(scope, target)
	if !ok || found == nil {
		return nil
	}
	return w.m.aliasTarget(found)
}

// positionalEnds returns the end at position i of each type owner specializes,
// which an end of owner at that position implicitly redefines (KerML 1.1 §8.3.3.3).
func (m *Model) positionalEnds(owner *symbols.Symbol, i int) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, sup := range m.allGenerals(owner) {
		if ends := m.endsOf(sup); i < len(ends) && ends[i] != nil {
			out = append(out, ends[i])
		}
	}
	return out
}

// allGenerals returns sym's direct supertypes and the library bases its kind
// implies, which DirectSupertypes leaves to ImplicitGenerals.
func (m *Model) allGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	out := append([]*symbols.Symbol(nil), m.DirectSupertypes(sym)...)
	for _, base := range m.ImplicitGenerals(sym) {
		if !slices.Contains(out, base) {
			out = append(out, base)
		}
	}
	return out
}

// endIndex is the position of end among the ends owner declares, or -1.
func endIndex(owner, end *symbols.Symbol) int {
	for i, owned := range ownedEnds(owner) {
		if owned == end {
			return i
		}
	}
	return -1
}

// enclosingScope is the scope the owner of sym is declared in: where a
// connector end's attachments and reference subsettings resolve.
func enclosingScope(sym *symbols.Symbol) *symbols.Scope {
	if owner := ownerSymbol(sym); owner != nil {
		return owner.OwnerScope
	}
	return sym.OwnerScope
}

// ownMultiplicity returns the range sym declares or its `multiplicity` member;
// a crossing multiplicity (`end [m] x`) is the cross feature's, not the end's.
func ownMultiplicity(sym *symbols.Symbol) (*ast.Multiplicity, *ast.MultiplicityDecl, bool) {
	var mult *ast.Multiplicity
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		mult = d.Multiplicity
	case *ast.Definition:
		mult = d.Multiplicity
	case *ast.CrossFeatureMember:
		mult = d.Multiplicity
	case *ast.SubjectMember:
		mult = d.Multiplicity
	}
	if mult != nil {
		return mult, nil, true
	}
	for _, member := range declMembers(sym) {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		if decl, ok := member.(*ast.MultiplicityDecl); ok {
			return nil, decl, true
		}
	}
	return nil, nil, false
}

// isExactlyOne reports whether r is 1..1; an unevaluable lower bound defaults to
// the upper, as `[1]` does (KerML 1.1 MultiplicityRange::hasBounds).
func isExactlyOne(r Range) bool {
	if !r.Upper.Known || r.Upper.Infinite || r.Upper.Value != 1 {
		return false
	}
	return !r.Lower.Known || (!r.Lower.Infinite && r.Lower.Value == 1)
}
