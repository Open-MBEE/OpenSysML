package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A valueless machine attribute still reaches the graph as machine-owned data.
func TestToStateGraph_KeepsValuelessAttributes(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				attribute count : Integer;
				entry; then active;
				state active;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if len(graph.Attributes) != 1 {
		t.Fatalf("attributes = %d, want 1", len(graph.Attributes))
	}
	if got := graph.Attributes[0]; got.Name != "count" || got.Value != nil {
		t.Errorf("attribute = %#v, want valueless count", got)
	}
}

// The textual `defer` notation reaches the graph as the deferral of the state
// declaring it, normalized the same way a transition trigger is.
func TestToStateGraph_DeferNotation(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy {
					defer Ping, setSpeed(value);
				}
				succession first start then busy;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	var busy *ast.StateNode
	for _, state := range graph.States {
		if state.Name == "busy" {
			busy = state
		}
	}
	if busy == nil {
		t.Fatal("state busy is not in the graph")
	}

	deferred := graph.Deferred[busy]
	if len(deferred) != 2 {
		t.Fatalf("expected busy to defer 2 events, got %d", len(deferred))
	}
	accept, ok := deferred[0].(*ast.AcceptEvent)
	if !ok {
		t.Fatalf("expected the first deferral to be a signal event, got %T", deferred[0])
	}
	if name := ast.SimpleName(accept.SignalType); name != "Ping" {
		t.Errorf("expected the deferred signal to be Ping, got %q", name)
	}
	call, ok := deferred[1].(*ast.CallEvent)
	if !ok {
		t.Fatalf("expected the second deferral to be a call event, got %T", deferred[1])
	}
	if name := ast.SimpleName(call.Operation); name != "setSpeed" {
		t.Errorf("expected the deferred call to be setSpeed, got %q", name)
	}
	if len(call.Parameters) != 1 || call.Parameters[0].Text != "value" {
		t.Errorf("expected the deferred call to carry the parameter value, got %v", call.Parameters)
	}
}

// A `defer` in the machine's own body has no state to defer for: the event
// would be retained for the whole run and never redelivered, so lowering
// reports it rather than dropping it.
func TestToStateGraph_DeferInMachineBodyIsReported(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				defer Ping;
				state busy;
				succession first start then busy;
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("expected an error for a defer in the state machine body")
	}
	if !strings.Contains(err.Error(), "defer must be declared inside a state") {
		t.Errorf("expected a placement error, got: %v", err)
	}
}

// History pseudostates written in the textual notation reach the graph with
// the kind their keyword names and with the composite state that declares them
// as their owner.
func TestToStateGraph_HistoryNotation(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state outer {
					state a;
					history resume;
					deep history resumeDeep;
					shallow history resumeShallow;
				}
				succession first start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	want := map[string]ast.PseudostateKind{
		"resume":        ast.PseudostateShallowHistory,
		"resumeDeep":    ast.PseudostateDeepHistory,
		"resumeShallow": ast.PseudostateShallowHistory,
	}
	for name, kind := range want {
		ps := pseudostateNamed(graph, name)
		if ps == nil {
			t.Errorf("pseudostate %q is not in the graph", name)
			continue
		}
		if ps.Kind != kind {
			t.Errorf("pseudostate %q: expected kind %v, got %v", name, kind, ps.Kind)
		}
		owner, ok := graph.PseudostateOwner[ps]
		if !ok || owner.Name != "outer" {
			t.Errorf("pseudostate %q: expected owner outer, got %v", name, owner)
		}
	}
}

// A succession out of the machine's own entry subaction is how standard
// notation names the state a machine starts in, so the target is the graph's
// initial state without an `initial` pseudostate being declared.
func TestToStateGraph_EntrySuccessionNamesInitialState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then off;
				state off;
				state on;
				succession first off then on;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if graph.Initial == nil {
		t.Fatal("expected the entry succession to name an initial state")
	}
	if graph.Initial.Name != "off" {
		t.Errorf("expected off to be the initial state, got %q", graph.Initial.Name)
	}
}

// A transition out of a named entry action names the state the machine starts
// in the same way, the action standing in for a start pseudostate.
func TestToStateGraph_EntryActionTransitionNamesInitialState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry action start { }
				transition start then off;
				state off;
				state on;
				transition off then on;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if graph.Initial == nil {
		t.Fatal("expected the entry action's transition to name an initial state")
	}
	if graph.Initial.Name != "off" {
		t.Errorf("expected off to be the initial state, got %q", graph.Initial.Name)
	}
	if got := len(graph.Transitions); got != 1 {
		t.Errorf("expected only off's transition to be an edge, got %d sources", got)
	}
}

// The succession spelling of the same designation names the initial state too.
func TestToStateGraph_EntryActionSuccessionNamesInitialState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry action begin { }
				succession first begin then off;
				state off;
				state on;
				succession first off then on;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if graph.Initial == nil {
		t.Fatal("expected the entry action's succession to name an initial state")
	}
	if graph.Initial.Name != "off" {
		t.Errorf("expected off to be the initial state, got %q", graph.Initial.Name)
	}
}

// A machine lowered from its scope tree alone — as a view rendering lowers it,
// without a resolver over the document — names the same initial state.
func TestToStateGraph_EntryActionNamesInitialStateFromScopeAlone(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				entry action begin { }
				transition begin then off;
				state off;
				state on;
				succession first off then on;
			}
		}
	`)
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := scopeOfNode(idx.DocumentRoot("test.sysml"), machine)
	if scope == nil {
		t.Fatal("the index has no scope for the machine")
	}

	graph, err := ToStateGraph(machine, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if graph.Initial == nil {
		t.Fatal("expected the entry action's transition to name an initial state")
	}
	if graph.Initial.Name != "off" {
		t.Errorf("expected off to be the initial state, got %q", graph.Initial.Name)
	}
}

// Each spelling that designates an initial state records it on the graph, which
// is where the runtime reads it from.
func TestToStateGraph_InitialDesignationIsRecordedOnTheGraph(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"named entry action", "entry action begin { }\ntransition first begin then off;"},
		{"anonymous entry", "entry; then off;"},
		{"initial pseudostate", "first i then off;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := stateUsageIn(t, `
				package test {
					state Machine {
						`+tc.body+`
						state off;
						state on;
						succession first off then on;
					}
				}
			`)
			graph, err := ToStateGraph(machine, nil)
			if err != nil {
				t.Fatalf("ToStateGraph: %v", err)
			}
			if graph.Initial == nil || graph.Initial.Name != "off" {
				t.Fatalf("expected off to be the initial state, got %v", graph.Initial)
			}
			if !graph.IsInitial(graph.Initial) {
				t.Error("expected the graph to report its initial state as initial")
			}
			for _, state := range graph.States {
				if state != graph.Initial && graph.IsInitial(state) {
					t.Errorf("state %q designated initial as well", state.Name)
				}
			}
		})
	}
}
