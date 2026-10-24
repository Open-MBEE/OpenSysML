package semantics

import "testing"

// A parameter list read while its owner's list is still being computed cuts
// the reading query short, so a closure built on it is not memoized either.
func TestParameterListGuardCutsDependentQueriesShort(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def Base { in x; out z; }
		action def Proc :> Base { in a; out c; }
	}`)
	p := sym(t, root, "P")
	proc := nested(t, p.Scope, "Proc")
	a := nested(t, p.Scope, "Proc", "a")
	wantX := nested(t, p.Scope, "Base", "x")
	// As parametersOf stands mid-computation for the owner.
	m.params[proc] = behaviorParameters{}
	m.computingParams[proc] = m.resolver.Enter()
	if got := m.AllRedefinedFeatures(a); len(got) != 0 {
		t.Fatalf("AllRedefinedFeatures(Proc::a) under the guard = %v, want the guard's empty answer", got)
	}
	m.resolver.Leave()
	delete(m.params, proc)
	delete(m.computingParams, proc)
	if got := m.AllRedefinedFeatures(a); len(got) != 1 || got[0] != wantX {
		t.Fatalf("AllRedefinedFeatures(Proc::a) after the guard = %v, want [Base::x]", got)
	}
}
