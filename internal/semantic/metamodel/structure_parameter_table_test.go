package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A general's parameter that redefines one an earlier general lent takes its place in the
// parameter table (KerML 8.3.3.1, 8.4.4.6), whichever order the generals are declared in,
// so the superseded parameter is neither listed nor redefined by position.
func TestStructureParameterTableSupersedesARedefinedInheritedParameter(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "p.sysml", `package P {
	action def A { in a; in a2; }
	action def B :> A { in b :>> a; }
	action def C :> A, B;
	action def D :> B, A;
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	element := func(name string) Element {
		syms := e.resolver.Index().LookupQualified(name)
		if len(syms) == 0 {
			t.Fatalf("no %s", name)
		}
		return structure.ElementFor(syms[0])
	}
	want := []ElementKey{element("P::B::b").Key(), element("P::A::a2").Key()}
	for _, owner := range []string{"P::C", "P::D"} {
		positional, _, ok := e.structureParameterTable(element(owner), make(map[ElementKey]bool))
		if !ok {
			t.Fatalf("%s: parameter table not derived", owner)
		}
		var got []ElementKey
		for _, p := range positional {
			got = append(got, p.element.Key())
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d parameters %v, want [B::b A::a2]", owner, len(got), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: parameter %d is %v, want %v", owner, i, got[i], want[i])
			}
		}
	}
}
