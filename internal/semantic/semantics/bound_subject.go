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
	// Walk every redefined branch: a redefining subject with no value inherits
	// a general's binding, even from a sibling branch the first branch misses.
	visited := map[*symbols.Symbol]bool{}
	parameters := map[*symbols.Symbol]bool{}
	var chain []*symbols.Symbol
	queue := []*symbols.Symbol{subject}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil || visited[current] {
			continue
		}
		visited[current] = true
		parameters[current] = true
		chain = append(chain, current)
		for _, general := range m.AllRedefinedFeatures(current) {
			if general != nil && !visited[general] {
				queue = append(queue, general)
			}
		}
	}
	for _, declared := range chain {
		binding, isDefault, isInitial, ok := ast.ValuePartOf(declared.Decl)
		if !ok || isDefault || isInitial {
			continue
		}
		// A plain binding is final: a general cannot replace it; a cast keeps
		// the value's declaration, not the type's.
		binding = boundValueOf(binding)
		if binding == nil {
			return boundSubjectResult{}
		}
		target, ok := m.resolver.ResolveTarget(declared.OwnerScope, binding)
		if !ok || target == nil {
			return boundSubjectResult{}
		}
		if alias, ok := m.resolver.ResolveAliasTarget(target); ok {
			target = alias
		}
		// No subject parameter in the chain is the declaration the case checks.
		if parameters[target] || symbols.SameElement(target, declared) {
			return boundSubjectResult{}
		}
		return boundSubjectResult{target: target, ok: true}
	}
	return boundSubjectResult{}
}

// boundValueOf peels casts off a binding expression: `boiler as Boiler` names
// boiler, not Boiler. `(as T)` alone names T for member lookup, so it binds nothing.
func boundValueOf(node ast.Node) ast.Node {
	for {
		switch n := node.(type) {
		case *ast.OperatorExpr:
			if n.Operator != ast.OpAs || len(n.Operands) == 0 || n.Operands[0] == nil {
				return node
			}
			node = n.Operands[0]
		case *ast.CastExpr:
			return nil
		default:
			return node
		}
	}
}
