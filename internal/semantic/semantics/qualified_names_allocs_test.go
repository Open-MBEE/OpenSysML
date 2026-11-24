package semantics

import (
	"slices"
	"testing"
)

func TestQualifiedNameOfANestedMemberAllocatesOnlyTheName(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part a { part inner { attribute x; } } }
		action def Act { in item payload; }
		action act : Act { in item; }
	}`)
	p := sym(t, root, "P")
	x := nested(t, p.Scope, "A", "a", "inner", "x")
	if got := m.QualifiedNameOf(x); got != "P::A::a::inner::x" {
		t.Fatalf("QualifiedNameOf(x) = %q", got)
	}
	names, ok := m.QualifiedNamesOf(x)
	if want := []string{"P", "A", "a", "inner", "x"}; !ok || !slices.Equal(names, want) {
		t.Fatalf("QualifiedNamesOf(x) = %v, %v; want %v", names, ok, want)
	}
	if allocs := testing.AllocsPerRun(50, func() { m.QualifiedNameOf(x) }); allocs > 1 {
		t.Errorf("QualifiedNameOf(x) allocates %v times, want the joined name alone", allocs)
	}
	positional := nested(t, p.Scope, "act").Scope.Declarations()[0]
	if got := m.QualifiedNameOf(positional); got != "P::act::payload" {
		t.Fatalf("QualifiedNameOf(positional parameter) = %q", got)
	}
}
