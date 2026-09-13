package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// `first a then b;` is the succession a -> b: it declares no node and marks no
// start, so the graph has no initial node and the executor infers the start.
func TestToActionGraph_FirstThenIsASuccession(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if graph.Initial != nil {
		t.Errorf("initial node = %s, want none", nodeDescription(graph.Initial))
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

// Any number of `first a then b;` successions stand beside `first start;`: start
// is the one initial node, and each succession leaves from the node it names.
func TestToActionGraph_FirstThenBesideFirstStart(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action a;
			action b;
			action c;
			first start;
			then fork f;
				then a;
				then b;
			first a then j;
			first b then j;
			join j;
			first j then c { doc /* c runs last */ }
			first c then done;
		}
	`)

	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok || initial.Name != "start" {
		t.Fatalf("initial node = %s, want the `first start;` marker", nodeDescription(graph.Initial))
	}
	want := map[string][]string{
		"start": {"f"},
		"f":     {"a", "b"},
		"a":     {"j"},
		"b":     {"j"},
		"j":     {"c"},
		"c":     {"done"},
	}
	for _, node := range graph.Nodes {
		var targets []string
		for _, edge := range graph.Edges[node] {
			targets = append(targets, nodeDescription(edge.Target))
		}
		if got, wanted := strings.Join(targets, ","), strings.Join(want[nodeDescription(node)], ","); got != wanted {
			t.Errorf("%s -> [%s], want [%s]", nodeDescription(node), got, wanted)
		}
	}
}

// The body a `first a then b { … }` carries is a succession's body: one holding
// annotations lowers to the same edge, one declaring a node is rejected as the
// body of `succession first a then b { … }` is.
func TestToActionGraph_FirstThenWithABody(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1 then s2 { doc /* s2 follows s1 */ }
		}
	`)
	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")
	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Errorf("s1 edges = %v, want [s2]", edges)
	}

	src := `
		action seq {
			action s1;
			action s2;
			first s1 then s2 { action s3; }
		}
	`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(usage, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported body") {
		t.Errorf("error = %v, want the succession's body rejected", err)
	}
}

// The one-ended `first s1;` names the node the flow starts at, so s1 is the
// graph's initial node and holds the succession written out of it.
func TestToActionGraph_FirstNamesADeclaredNode(t *testing.T) {
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
	if initial.Name != "start" {
		t.Errorf("initial node name = %q, want %q", initial.Name, "start")
	}
	if edges := graph.Edges[initial]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "s1") {
		t.Errorf("initial edges = %v, want [s1]", edges)
	}
}

// A one-ended `first done;` names a final node, stating a flow that ends where
// it starts, so lowering rejects it.
func TestToActionGraph_FirstNamesAFinalNode(t *testing.T) {
	src := `
		action seq {
			action s1;
			done;
			first done;
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
