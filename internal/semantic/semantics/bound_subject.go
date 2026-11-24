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
	// The owned subject comes first, then the ones it redefines: a redefining
	// subject with no value of its own inherits the general's binding.
	visited := map[*symbols.Symbol]bool{}
	parameters := map[*symbols.Symbol]bool{}
	var chain []*symbols.Symbol
	for current := subject; current != nil && !visited[current]; {
		visited[current] = true
		parameters[current] = true
		chain = append(chain, current)
		var next *symbols.Symbol
		for _, general := range m.AllRedefinedFeatures(current) {
			if general != nil && !visited[general] {
				next = general
				break
			}
		}
		current = next
	}
	for _, declared := range chain {
		binding, isDefault, isInitial, ok := ast.ValuePartOf(declared.Decl)
		if !ok || isDefault || isInitial {
			continue
		}
		target, ok := m.resolver.ResolveTarget(declared.OwnerScope, binding)
		if !ok || target == nil {
			continue
		}
		if alias, ok := m.resolver.ResolveAliasTarget(target); ok {
			target = alias
		}
		// No subject parameter in the chain is the declaration the case checks.
		if parameters[target] || symbols.SameElement(target, declared) {
			continue
		}
		return boundSubjectResult{target: target, ok: true}
	}
	return boundSubjectResult{}
}
