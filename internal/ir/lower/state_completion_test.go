package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A transition to `done` lowers to a completion vertex of the machine: the
// graph, not the syntax, records which state completes it.
func TestToStateGraph_DoneLowersToACompletionVertex(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy;
				succession first start then busy;
				transition first busy accept Stop then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	completion := stateNamed(graph, ast.DoneFeature)
	if completion == nil {
		t.Fatal("no completion vertex in the graph")
	}
	if !graph.Completes(completion) {
		t.Error("the completion vertex does not complete the machine")
	}
	if busy := stateNamed(graph, "busy"); busy == nil || graph.Completes(busy) {
		t.Error("an ordinary state completes the machine")
	}
	if len(graph.Transitions[stateNamed(graph, "busy")]) != 1 {
		t.Errorf("expected busy to keep its transition, got %v", graph.Transitions[stateNamed(graph, "busy")])
	}
}

// Two transitions reaching `done` in one machine share its single completion
// vertex rather than each synthesizing one.
func TestToStateGraph_CompletionVertexIsShared(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy;
				succession first start then busy;
				transition first start accept Abort then done;
				transition first busy accept Stop then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	completions := 0
	for _, state := range graph.States {
		if graph.Completes(state) {
			completions++
		}
	}
	if completions != 1 {
		t.Errorf("expected one completion vertex, got %d", completions)
	}
}

// Each region of an orthogonal machine completes on its own, so each owns a
// completion vertex of that region.
func TestToStateGraph_EachRegionOwnsItsCompletion(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept First then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart accept Second then done;
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	regions := map[string]bool{}
	for _, state := range graph.States {
		if !graph.Completes(state) {
			continue
		}
		region := graph.RegionOf[state]
		if region == nil {
			t.Fatal("a completion vertex belongs to no region")
		}
		regions[region.Name] = true
	}
	if len(regions) != 2 {
		t.Errorf("expected a completion in each region, got %v", regions)
	}
}

// A machine declaring its own `done` state reaches that state, so no completion
// vertex is synthesized for it.
func TestToStateGraph_DeclaredDoneIsNotACompletion(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state done;
				transition first start accept Stop then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	for _, state := range graph.States {
		if graph.Completes(state) {
			t.Errorf("state %q completes the machine, want the declared state", state.Name)
		}
	}
}
