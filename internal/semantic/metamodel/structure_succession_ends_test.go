package metamodel

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A succession's implied source end (the occurrence before a `then x;`, a transition's `first s`)
// owns no ReferenceSubsetting, yet relates the feature it stands for (KerML 8.3.4.1 Connector::relatedFeature).
func TestSymbolStructureRelatesASuccessionThroughItsImpliedSourceEnd(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "b13.sysml", `package B13 {
	item def Sig;
	action def Shorthand { action a1; action a2; first a1; then a2; }
	action def NodeEnds { action a1; action a2; first a1; then accept Sig; then action a3; }
	action def Guarded { action a1; decide d; succession g first d if true then a1; }
	state def Machine { state s1; state s2; transition t first s1 accept Sig then s2; }
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	pkg := testSymbol(t, root, "B13")
	var successions func(owner *symbols.Symbol) []*symbols.Symbol
	successions = func(owner *symbols.Symbol) []*symbols.Symbol {
		var out []*symbols.Symbol
		for _, m := range evaluator.model.OwnedMemberships(owner) {
			if m.Implicit == nil || m.Implicit.Member == nil {
				continue
			}
			switch evaluator.model.MetaclassOf(m.Implicit.Member).Name {
			case "SuccessionAsUsage":
				out = append(out, m.Implicit.Member)
			case "TransitionUsage":
				out = append(out, successions(m.Implicit.Member)...)
			}
		}
		return out
	}
	name := func(value Value) string {
		if value.Kind != ElementValue {
			return "<none>"
		}
		sym, ok := structure.SymbolOf(value.Element)
		if !ok || sym == nil {
			return "<no symbol>"
		}
		if sym.Name == "" {
			return "<unnamed>"
		}
		return symbols.FQNOf(sym)
	}
	want := map[string][]string{
		"Shorthand": {"B13::Shorthand::a1 -> B13::Shorthand::a2"},
		"NodeEnds":  {"B13::NodeEnds::a1 -> <unnamed>", "<unnamed> -> B13::NodeEnds::a3"},
		"Guarded":   {"B13::Guarded::d -> B13::Guarded::a1"},
		"Machine":   {"B13::Machine::s1 -> B13::Machine::s2"},
	}
	for def, edges := range want {
		var got []string
		for _, succession := range successions(testSymbol(t, pkg.Scope, def)) {
			element := structure.ElementFor(succession)
			source, ok := e.Property(element, "Connector", "sourceFeature")
			if !ok || !source.Success {
				t.Fatalf("%s: sourceFeature = %v, %v", def, source, ok)
			}
			target, ok := e.Property(element, "Connector", "targetFeature")
			if !ok || !target.Success || target.Kind != SequenceValue || len(target.Values) != 1 {
				t.Fatalf("%s: targetFeature = %v, %v, want one feature", def, target, ok)
			}
			got = append(got, name(source)+" -> "+name(target.Values[0]))
		}
		if strings.Join(got, "; ") != strings.Join(edges, "; ") {
			t.Errorf("%s relates %v, want %v", def, got, edges)
		}
	}
}
