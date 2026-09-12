package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestToActionGraph_ExplicitSuccessionUsage(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action compute;
			succession first start then compute;
			succession first compute then done;
		}
	`)

	start := namedActionNode(t, graph, "start")
	compute := nodeNamed(t, graph, "compute")
	done := namedActionNode(t, graph, "done")

	if _, ok := start.(*ast.InitialNode); !ok {
		t.Fatalf("start node = %T, want implied initial node", start)
	}
	if _, ok := done.(*ast.FinalNode); !ok {
		t.Fatalf("done node = %T, want implied final node", done)
	}
	if edges := graph.Edges[start]; len(edges) != 1 || edges[0].Target != compute {
		t.Fatalf("start edges = %v, want [compute]", edges)
	}
	if edges := graph.Edges[compute]; len(edges) != 1 || edges[0].Target != done {
		t.Fatalf("compute edges = %v, want [done]", edges)
	}
	if _, ok := graph.Edges[start][0].Decl.(*ast.Usage); !ok {
		t.Error("explicit succession was not recorded as its Usage declaration")
	}
}

func TestToActionGraph_ImpliedEndpointsOnControlFlow(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			first start;
			decide firstCheck;
			succession first start then firstCheck;
			if true then done;
			else secondCheck;
			decide secondCheck;
			if false then done;
			else done;
		}
	`)

	start := namedActionNode(t, graph, "start")
	done := namedActionNode(t, graph, "done")
	firstCheck := namedActionNode(t, graph, "firstCheck")
	secondCheck := namedActionNode(t, graph, "secondCheck")

	if edges := graph.Edges[firstCheck]; len(edges) != 2 || edges[0].Target != done || edges[1].Target != secondCheck {
		t.Fatalf("firstCheck edges = %v, want [done secondCheck]", edges)
	}
	if edges := graph.Edges[secondCheck]; len(edges) != 2 || edges[0].Target != done || edges[1].Target != done {
		t.Fatalf("secondCheck edges = %v, want [done done]", edges)
	}
	if graph.Edges[secondCheck][0].Guard == nil {
		t.Fatal("secondCheck guarded done edge lost its guard")
	}
	if graph.Edges[secondCheck][1].Guard != nil {
		t.Fatal("secondCheck else done edge unexpectedly gained a guard")
	}
	if graph.Edges[secondCheck][0].Decl == nil || graph.Edges[secondCheck][1].Decl == nil {
		t.Fatal("secondCheck done edges lost their declarations")
	}
	if graph.Edges[secondCheck][0].Decl == graph.Edges[secondCheck][1].Decl {
		t.Fatal("secondCheck done edges share a declaration")
	}
	if len(graph.Finals) != 1 || graph.Finals[0] != done {
		t.Fatalf("final nodes = %v, want one shared implied done node", graph.Finals)
	}
	if edges := graph.Edges[start]; len(edges) != 1 || edges[0].Target != firstCheck {
		t.Fatalf("start edges = %v, want [firstCheck]", edges)
	}
}

func TestToActionGraph_ImpliedEndpointOnInitialSuccessor(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			first start then done;
		}
	`)

	start := namedActionNode(t, graph, "start")
	done := namedActionNode(t, graph, "done")
	if edges := graph.Edges[start]; len(edges) != 1 || edges[0].Target != done {
		t.Fatalf("start edges = %v, want [done]", edges)
	}
	if len(graph.Finals) != 1 || graph.Finals[0] != done {
		t.Fatalf("final nodes = %v, want one shared implied done node", graph.Finals)
	}
}

func TestToActionGraph_ExplicitSuccessionFeatureChain(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action source {
				attribute pin : Integer = 0;
			}
			action target;
			succession first source.pin then target;
		}
	`)

	source := nodeNamed(t, graph, "source")
	target := nodeNamed(t, graph, "target")
	if edges := graph.Edges[source]; len(edges) != 1 || edges[0].Target != target {
		t.Fatalf("source edges = %v, want [target]", edges)
	}
}

func TestToActionGraph_ExplicitSuccessionWithoutInitial(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action alpha;
			action beta;
			succession first alpha then beta;
		}
	`)

	if graph.Initial != nil {
		t.Fatalf("initial node = %v, want no initial node", graph.Initial)
	}
}

func TestToActionGraph_ExplicitSuccessionUndefinedEndpoint(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		action seq {
			action compute;
			succession first missing then compute;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	action := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(action, nil)
	if err == nil || !strings.Contains(err.Error(), "action succession references undefined source node") {
		t.Fatalf("error = %v, want an explicit succession source diagnostic", err)
	}
}

func TestToActionGraph_ExplicitSuccessionUnsupportedMultiplicity(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		action seq {
			action first;
			action second;
			succession [1] first first then second;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	action := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(action, nil)
	if err == nil || !strings.Contains(err.Error(), "action succession has unsupported multiplicity") {
		t.Fatalf("error = %v, want an explicit succession multiplicity diagnostic", err)
	}
}

// A succession body holding only annotations declares nothing the flow depends
// on, so it lowers; one declaring a feature does not.
func TestToActionGraph_ExplicitSuccessionBody(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action alpha;
			action beta;
			succession first alpha then beta { @Layout { x = 1; } doc /* routed */ }
		}
	`)
	alpha := nodeNamed(t, graph, "alpha")
	if edges := graph.Edges[alpha]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "beta") {
		t.Fatalf("alpha edges = %v, want [beta]", edges)
	}

	p := parser.New(source.New("test.sysml", []byte(`
		action seq {
			action alpha;
			action beta;
			succession first alpha then beta { attribute weight : Integer; }
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	action := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(action, nil)
	if err == nil || !strings.Contains(err.Error(), "action succession has unsupported body") {
		t.Fatalf("error = %v, want an explicit succession body diagnostic", err)
	}
}

func namedActionNode(t *testing.T, graph *ActionGraph, name string) ast.Node {
	t.Helper()
	for _, node := range graph.Nodes {
		if getNodeName(node) == name {
			return node
		}
	}
	t.Fatalf("node %s not found in graph", name)
	return nil
}

// StartFlow gives an action performed whole its one unpreceded step to start at,
// keeps an explicit start, and leaves an ambiguous or cyclic flow without one.
func TestStartFlow(t *testing.T) {
	for _, tc := range []struct {
		name, body, start string
	}{
		{"one unpreceded step", `action alpha; then action beta;`, "alpha"},
		{"explicit first", `action alpha; action beta; first beta then alpha;`, "beta"},
		{"explicit start node", `first start; then action alpha; action beta;`, "start"},
		{"two unpreceded steps", `action alpha; action beta; action gamma; succession first alpha then gamma; succession first beta then gamma;`, ""},
		{"a cycle", `action alpha; action beta; succession first alpha then beta; succession first beta then alpha;`, ""},
	} {
		graph := actionGraphFor(t, `action seq { `+tc.body+` }`)
		StartFlow(graph)
		if tc.start == "" {
			if graph.Initial != nil {
				t.Errorf("%s: initial node = %v, want none", tc.name, graph.Initial)
			}
			continue
		}
		if tc.start == "start" {
			if _, ok := graph.Initial.(*ast.InitialNode); !ok {
				t.Errorf("%s: initial node = %T, want the start node", tc.name, graph.Initial)
			}
			continue
		}
		if graph.Initial != nodeNamed(t, graph, tc.start) {
			t.Errorf("%s: initial node = %v, want %s", tc.name, graph.Initial, tc.start)
		}
	}
}
