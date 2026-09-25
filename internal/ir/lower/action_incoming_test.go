package lower

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Incoming answers the successions into a node as walking every edge would find
// them, in node order, from one index built on the first call.
func TestActionGraphIncomingMatchesTheEdges(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action a;
			action b;
			action c;
			action d;
			first start;
			then a;
			first a then c;
			first b then c;
			first c then d;
			first a then d;
		}
	`)
	want := map[ast.Node][]ActionEdge{}
	for _, source := range graph.Nodes {
		for _, edge := range graph.Edges[source] {
			want[edge.Target] = append(want[edge.Target], edge)
		}
	}
	d := nodeNamed(t, graph, "d")
	first := graph.Incoming(d)
	for _, name := range []string{"a", "b", "c", "d"} {
		node := nodeNamed(t, graph, name)
		if got := graph.Incoming(node); !reflect.DeepEqual(got, want[node]) {
			t.Errorf("Incoming(%s) = %v, want %v", name, got, want[node])
		}
	}
	if len(graph.Incoming(nodeNamed(t, graph, "b"))) != 0 {
		t.Error("b has incoming successions, want none")
	}
	if len(first) != 2 || len(graph.Incoming(nodeNamed(t, graph, "c"))) != 2 {
		t.Errorf("d has %d incoming successions, want 2 (c has %d, want 2)", len(first), len(graph.Incoming(nodeNamed(t, graph, "c"))))
	}
	if again := graph.Incoming(d); &again[0] != &first[0] {
		t.Error("a second call built the incoming successions anew")
	}
}
