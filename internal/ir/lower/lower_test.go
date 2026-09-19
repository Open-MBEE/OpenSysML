package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestToActionGraph_Simple(t *testing.T) {
	src := `
		action test {
			first start;
			done;
			succession first start then done;
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	// Find action usage
	var actionUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if usage, ok := membership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
				actionUsage = usage
				break
			}
		}
	}

	if actionUsage == nil {
		t.Fatal("no action usage found")
	}

	graph, err := ToActionGraph(actionUsage, nil)
	if err != nil {
		t.Fatalf("ToActionGraph failed: %v", err)
	}

	// Validate graph structure
	if graph.Initial == nil {
		t.Error("graph has no initial node")
	}

	if len(graph.Finals) == 0 {
		t.Error("graph has no final nodes")
	}

	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes (initial + final), got %d", len(graph.Nodes))
	}

	// Validate edges: initial → final
	edges := graph.Edges[graph.Initial]
	if len(edges) != 1 {
		t.Errorf("initial node should have 1 edge, got %d", len(edges))
	}
}

func TestToStateGraph_Simple(t *testing.T) {
	src := `
		package test {
			state Machine {
				entry; then start;
				state start;
				state idle;
				
				succession first start then idle;
				succession first idle then done;
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %v", d)
		}
		t.Fatalf("parse errors")
	}

	// Find state usage in package
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	if stateUsage == nil {
		t.Fatal("no state usage found")
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Validate graph structure
	if graph.Initial == nil {
		t.Error("graph has no initial state")
	}

	if len(graph.States) < 2 {
		t.Errorf("expected at least 2 states, got %d", len(graph.States))
	}

	// Find initial state
	initialFound := false
	for _, state := range graph.States {
		if graph.IsInitial(state) {
			initialFound = true
			t.Logf("Found initial state: %s", state.Name)
		}
	}

	if !initialFound {
		t.Error("no state marked as initial")
	}
}

func TestToActionGraph_ForkJoinMergeDecision(t *testing.T) {
	src := `
		action parallel {
			first start;
			fork f;
			action a1;
			action a2;
			join j;
			done;
			
			succession first start then f;
			succession first f then a1;
			succession first f then a2;
			succession first a1 then j;
			succession first a2 then j;
			succession first j then done;
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var actionUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if usage, ok := membership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
				actionUsage = usage
				break
			}
		}
	}

	graph, err := ToActionGraph(actionUsage, nil)
	if err != nil {
		t.Fatalf("ToActionGraph failed: %v", err)
	}

	// Should have: initial, fork, 2 actions, join, final = 6 nodes
	if len(graph.Nodes) < 6 {
		t.Errorf("expected at least 6 nodes, got %d", len(graph.Nodes))
	}

	// Fork should have 2 outgoing edges
	var forkNode ast.Node
	for _, node := range graph.Nodes {
		if fn, ok := node.(*ast.ForkNode); ok {
			forkNode = fn
			break
		}
	}

	if forkNode == nil {
		t.Fatal("no fork node found")
	}

	if len(graph.Edges[forkNode]) != 2 {
		t.Errorf("fork should have 2 outgoing edges, got %d", len(graph.Edges[forkNode]))
	}
}

func TestToStateGraph_Regions(t *testing.T) {
	src := `
		package test {
			state TrafficLight parallel {
				state vehicle {
					entry; then v_start;
					state v_start;
					state Red;
					state Green;
					succession first v_start then Red;
					succession first Red then Green;
				}
				
				state pedestrian {
					entry; then p_start;
					state p_start;
					state Walk;
					state DontWalk;
					succession first p_start then Walk;
					succession first Walk then DontWalk;
				}
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Should have 2 regions with initial states
	if len(graph.RegionInitials) != 2 {
		t.Errorf("expected 2 region initials, got %d", len(graph.RegionInitials))
	}

	// Should have 4 states (Red, Green, Walk, DontWalk) + 2 initials = 6
	if len(graph.States) < 6 {
		t.Errorf("expected at least 6 states, got %d", len(graph.States))
	}
}

// pseudostateNamed is the graph's pseudostate with that simple name, or nil. The
// graph keys none by name, since two regions may declare same-named ones.
func pseudostateNamed(graph *StateGraph, name string) *ast.PseudostateNode {
	for _, ps := range graph.Pseudostates {
		if ps.Name == name {
			return ps
		}
	}
	return nil
}

func TestToStateGraph_Pseudostates(t *testing.T) {
	src := `
		package test {
			state Router {
				entry; then start;
				state start;
				choice c;
				state lowPriority;
				state highPriority;
				
				succession first start then c;
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Should have 1 choice pseudostate
	if len(graph.Pseudostates) != 1 {
		t.Errorf("expected 1 pseudostate, got %d", len(graph.Pseudostates))
	}

	choiceNode := pseudostateNamed(graph, "c")
	if choiceNode == nil {
		t.Fatal("choice pseudostate 'c' not found")
	}

	if choiceNode.Kind != ast.PseudostateChoice {
		t.Errorf("expected choice pseudostate, got %v", choiceNode.Kind)
	}
}
