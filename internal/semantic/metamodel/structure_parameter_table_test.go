package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A general's parameter redefining one an earlier general lent takes its place in the table
// (KerML 8.3.3.1, 8.4.4.6) in either general order, so the superseded one is listed nowhere.
func TestStructureParameterTableSupersedesARedefinedInheritedParameter(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "p.sysml", `package P {
	action def A { in a; in a2; }
	action def B :> A { in b :>> a; }
	action def C :> A, B;
	action def D :> B, A;
	action def A2 { in a; }
	action def B2 :> A2 { out b; }
	action def E :> A2, B2;
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
	// A later general's parameter of another direction at the position is no redefinition.
	positional, _, ok := e.structureParameterTable(element("P::E"), make(map[ElementKey]bool))
	if !ok {
		t.Fatal("P::E: parameter table not derived")
	}
	if len(positional) != 2 || positional[0].element.Key() != element("P::A2::a").Key() || positional[1].element.Key() != element("P::B2::b").Key() {
		t.Errorf("P::E: parameters %v, want [A2::a B2::b]", positional)
	}
}
