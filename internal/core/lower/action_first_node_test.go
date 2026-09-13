package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// `first a then b;` names the node the flow starts at, so a is the graph's
// initial node and holds the succession out of it.
func TestToActionGraph_FirstNamesADeclaredNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if graph.Initial != s1 {
		t.Errorf("initial node = %s, want s1", nodeDescription(graph.Initial))
	}
	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Errorf("s1 edges = %v, want [s2]", edges)
	}
	for _, node := range graph.Nodes {
		if _, ok := node.(*ast.InitialNode); ok {
			t.Error("the first end was kept as an initial node of its own")
		}
	}
}

// The same start written as its own member: the succession out of the first node
// is a separate member and still leaves from the named node.
func TestToActionGraph_FirstNamesADeclaredNodeSplit(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1;
			succession first s1 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if graph.Initial != s1 {
		t.Errorf("initial node = %s, want s1", nodeDescription(graph.Initial))
	}
	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Errorf("s1 edges = %v, want [s2]", edges)
	}
}

// A `first` end naming no declared node still declares an initial node of its own.
func TestToActionGraph_FirstDeclaresItsOwnInitialNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			first start;
			succession first start then s1;
		}
	`)

	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok {
		t.Fatalf("initial node = %T, want *ast.InitialNode", graph.Initial)
	}
	if initial.Name() != "start" {
		t.Errorf("initial node name = %q, want %q", initial.Name(), "start")
	}
	if edges := graph.Edges[initial]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "s1") {
		t.Errorf("initial edges = %v, want [s1]", edges)
	}
}

// A `first` end naming a final node states a flow that ends where it starts.
func TestToActionGraph_FirstNamesAFinalNode(t *testing.T) {
	src := `
		action seq {
			action s1;
			done;
			first done then s1;
		}
	`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(usage, nil)
	if err == nil {
		t.Fatal("a first end naming a final node lowered without an error")
	}
	if !strings.Contains(err.Error(), "final node done") {
		t.Errorf("error = %q, want it to name the final node", err)
	}
}

// `first s1 if c then s2;` carries the guard the member states onto the lowered
// succession, which is what the executor evaluates before traversing it.
func TestToActionGraph_GuardOnTheSuccessionOutOfTheFirstNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			attribute x : Integer = 0;
			action s1;
			action s2;
			first s1 if x > 0 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Fatalf("s1 edges = %v, want [s2]", edges)
	}
	if graph.Edges[s1][0].Guard == nil {
		t.Error("the guard the member states was not carried onto the succession")
	}
}

// The same guard on a succession written as its own member out of an ordinary
// action node (`succession first s1 if c then s2;`).
func TestToActionGraph_GuardOnASuccessionOutOfAnActionNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			attribute x : Integer = 0;
			action s1;
			action s2;
			first s1;
			succession first s1 if x > 0 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Fatalf("s1 edges = %v, want [s2]", edges)
	}
	if graph.Edges[s1][0].Guard == nil {
		t.Error("the guard the member states was not carried onto the succession")
	}
}

func nodeDescription(node ast.Node) string {
	if name := getNodeName(node); name != "" {
		return name
	}
	return "an unnamed node"
}
