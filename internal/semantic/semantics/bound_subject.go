package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// boundSubjectResult is one memoized answer of BoundSubjectOf. ok distinguishes
// a case with a binding from one without; target is nil in the latter.
type boundSubjectResult struct {
	target *symbols.Symbol
	ok     bool
}

// BoundSubjectOf returns the declaration a case's subject binds (`subject s = X;`),
// never the subject parameter itself; false is absence. Memoized per element.
func (m *Model) BoundSubjectOf(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	if cached, seen := m.boundSubjects[sym]; seen {
		return cached.target, cached.ok
	}
	if m.computingBoundSubjects[sym] {
		return nil, false
	}
	if m.computingBoundSubjects == nil {
		m.computingBoundSubjects = make(map[*symbols.Symbol]bool)
	}
	if m.boundSubjects == nil {
		m.boundSubjects = make(map[*symbols.Symbol]boundSubjectResult)
	}
	m.computingBoundSubjects[sym] = true
	result := m.boundSubjectOf(sym)
	delete(m.computingBoundSubjects, sym)
	m.boundSubjects[sym] = result
	return result.target, result.ok
}

func (m *Model) boundSubjectOf(sym *symbols.Symbol) boundSubjectResult {
	subject := m.SubjectParameterOf(sym)
	if subject == nil || m.resolver == nil {
		return boundSubjectResult{}
	}
	binding, isDefault, isInitial, ok := ast.ValuePartOf(subject.Decl)
	if !ok || isDefault || isInitial {
		return boundSubjectResult{}
	}
	target, ok := m.resolver.ResolveTarget(subject.OwnerScope, binding)
	if !ok || target == nil {
		return boundSubjectResult{}
	}
	if alias, ok := m.resolver.ResolveAliasTarget(target); ok {
		target = alias
	}
	// The subject parameter is not the declaration it binds. Never answer it in
	// place of one: an unbound case must stay absent, not borrow another element.
	if symbols.SameElement(target, subject) {
		return boundSubjectResult{}
	}
	return boundSubjectResult{target: target, ok: true}
}
