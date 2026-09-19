package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A namespace succession is retained by parsing but has no behavior body to lower.
func TestNamespaceSuccessionHasNoTokenFlowOrNeighborGraphAttachment(t *testing.T) {
	const src = `
		namespace N {
			action def a {
				action x;
			}
			action def b {
				action y;
			}
			first a::x then b::y;
			action neighbor {
				first start;
				done;
				succession first start then done;
			}
		}
	`

	file := source.New("namespace-succession.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var succession *ast.Usage
	var neighbor *ast.Usage
	var walk func([]ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *ast.Membership:
				walk([]ast.Node{n.Member})
			case *ast.Namespace:
				walk(n.Members)
			case *ast.Usage:
				if n.Kind == ast.UsageSuccession && succession == nil {
					succession = n
				}
				name, _ := ast.EffectiveName(n)
				if name == "neighbor" {
					neighbor = n
				}
				walk(n.Members)
			}
		}
	}
	walk(root.Members)
	if succession == nil {
		t.Fatal("namespace succession was not retained in the AST")
	}
	if len(succession.ConnectorEnds) != 2 {
		t.Fatalf("namespace succession has %d connector ends, want 2", len(succession.ConnectorEnds))
	}
	if neighbor == nil {
		t.Fatal("neighbor action was not retained in the AST")
	}

	graph, err := ToActionGraph(neighbor, nil)
	if err != nil {
		t.Fatalf("ToActionGraph(neighbor): %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("neighbor graph has %d nodes, want only its start and finish nodes", len(graph.Nodes))
	}
	for source, edges := range graph.Edges {
		for _, edge := range edges {
			if edge.Decl == succession {
				t.Fatalf("namespace succession was attached to neighbor graph from %T to %T", source, edge.Target)
			}
		}
	}
}
