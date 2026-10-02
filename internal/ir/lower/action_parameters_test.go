package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An action's own directed parameters are lowered as its Parameters, in order,
// and nothing else of its members is.
func TestActionGraphParameters(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			in bread : Integer;
			attribute n : Integer = 1;
			out toast : Integer;
			inout both : Integer;
			return r : Integer;
			first start; then done;
		}
	`)
	want := []struct {
		name      string
		direction ast.FeatureDirection
		result    bool
	}{{"bread", ast.DirIn, false}, {"toast", ast.DirOut, false}, {"both", ast.DirInOut, false}, {"r", ast.DirOut, true}}
	if len(graph.Parameters) != len(want) {
		t.Fatalf("parameters = %+v, want %d", graph.Parameters, len(want))
	}
	for i, w := range want {
		got := graph.Parameters[i]
		if got.Name != w.name || got.Direction != w.direction || got.IsResult != w.result || got.Node == nil {
			t.Errorf("parameter %d = %+v, want %s/%v result=%v", i, got, w.name, w.direction, w.result)
		}
	}
}

// A node's pin valued by a name is a ValueBinding: to the action's own parameter
// where it names one, bare or qualified; to another node's pin where it names
// that; to neither where it names an attribute, and no binding at all for a
// literal. An explicit bind to the action's parameter names it too.
func TestActionGraphValueBindings(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def A {
			in bread : Integer;
			out toast : Integer;
			attribute n : Integer = 1;
			action heat { in b = bread; out t : Integer; }
			action pack { in t = heat.t; in q = A::bread; in k = n; in lit = 3; out boxed : Integer; }
			bind pack.boxed = toast;
			first start; then heat; then pack; then done;
		}
	`, "A")
	heat, pack := nodeNamed(t, graph, "heat"), nodeNamed(t, graph, "pack")
	type end struct {
		node      ast.Node
		pin       string
		otherNode ast.Node
		otherPin  string
		parameter string
	}
	want := []end{
		{heat, "b", nil, "", "bread"},
		{pack, "t", heat, "t", ""},
		{pack, "q", nil, "", "bread"},
		{pack, "k", nil, "", ""},
	}
	if len(graph.ValueBindings) != len(want) {
		t.Fatalf("value bindings = %+v, want %d", graph.ValueBindings, len(want))
	}
	for i, w := range want {
		got := graph.ValueBindings[i]
		if got.Node != w.node || got.Pin != w.pin || got.OtherNode != w.otherNode || got.OtherPin != w.otherPin || got.OtherParameter != w.parameter || !got.FromValue {
			t.Errorf("value binding %d = node %s pin %s other %v.%s parameter %q, want %+v", i, getNodeName(got.Node), got.Pin, got.OtherNode, got.OtherPin, got.OtherParameter, w)
		}
	}
	if len(graph.Bindings) != 1 || graph.Bindings[0].Node != pack || graph.Bindings[0].Pin != "boxed" || graph.Bindings[0].OtherParameter != "toast" {
		t.Errorf("bindings = %+v, want pack.boxed to the parameter toast", graph.Bindings)
	}
}
