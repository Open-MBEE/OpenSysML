package lower

import (
	"strings"
	"testing"
)

// The inline body of an entry, do or exit behavior stating successions or
// control nodes is lowered to the token flow it states, as a standalone action's
// body is; one stating none stays a block of statements.
func TestStateBehaviorBodyStatingAFlowIsLoweredToAnActionGraph(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			action def A;
			state Machine {
				entry; then s;
				state s {
					entry action prep {
						first start;
						then action a : A;
						then done;
					}
					do action ops {
						action a : A;
						action b : A;
						first a then b;
					}
					exit action wrap { action c : A; action d : A; }
				}
				succession first s then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	s := stateNamed(graph, "s")
	if s == nil {
		t.Fatal("state s was not lowered")
	}
	behaviors := graph.Behaviors[s]
	for _, tc := range []struct {
		kind string
		body []Statement
		into string // the node the flow's first succession leads into
	}{
		{"entry", behaviors.Entry[0].Body, "a"},
		{"do", behaviors.Do[0].Body, "b"},
	} {
		if len(tc.body) != 1 {
			t.Fatalf("%s body = %d statements, want the one block", tc.kind, len(tc.body))
		}
		block, ok := tc.body[0].(Block)
		if !ok {
			t.Fatalf("%s body is %T, want Block", tc.kind, tc.body[0])
		}
		if block.Graph == nil || !block.Stated || !block.Own {
			t.Fatalf("%s body block: Graph=%v Stated=%v Own=%v, want the stated flow of its own",
				tc.kind, block.Graph != nil, block.Stated, block.Own)
		}
		if block.Graph.Initial == nil {
			t.Fatalf("%s body flow has no initial node", tc.kind)
		}
		edges := block.Graph.Edges[block.Graph.Initial]
		if len(edges) != 1 || edges[0].Target != nodeNamed(t, block.Graph, tc.into) {
			t.Errorf("%s body flow's first succession = %v, want one into %s", tc.kind, edges, tc.into)
		}
	}
	block, ok := behaviors.Exit[0].Body[0].(Block)
	if !ok {
		t.Fatalf("exit body is %T, want Block", behaviors.Exit[0].Body[0])
	}
	if block.Stated {
		t.Errorf("exit body stating no flow was lowered as a stated flow")
	}
}

// A body whose successions cannot be built into a flow is lowered to an
// Unsupported statement naming the reason, so the runtime reports it.
func TestStateBehaviorBodyWithADanglingSuccessionIsUnsupported(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			action def A;
			state Machine {
				entry; then s;
				state s {
					do action ops {
						first start;
						then action a : A;
						succession a then missing;
					}
				}
				succession first s then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	body := graph.Behaviors[stateNamed(graph, "s")].Do[0].Body
	if len(body) != 1 {
		t.Fatalf("do body = %d statements, want 1", len(body))
	}
	unsupported, ok := body[0].(Unsupported)
	if !ok {
		t.Fatalf("do body is %T, want Unsupported", body[0])
	}
	if !strings.Contains(unsupported.Description, `"missing"`) {
		t.Errorf("description %q does not name the undefined target", unsupported.Description)
	}
}

// A behavior body whose successions leave one step unpreceded starts there, as a
// case body's flow does, so `first` is needed only where the start is ambiguous.
func TestStateBehaviorBodyStartsAtItsOneUnprecededStep(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			action def A;
			state Machine {
				entry; then s;
				state s {
					do action ops {
						action a : A;
						then action b : A;
					}
				}
				succession first s then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	block, ok := graph.Behaviors[stateNamed(graph, "s")].Do[0].Body[0].(Block)
	if !ok || block.Graph == nil {
		t.Fatalf("do body is not a stated flow: %#v", graph.Behaviors[stateNamed(graph, "s")].Do[0].Body)
	}
	if block.Graph.Initial != nodeNamed(t, block.Graph, "a") {
		t.Errorf("initial node = %v, want the unpreceded step a", block.Graph.Initial)
	}
}

// Two unpreceded steps, or a cycle, leave the start unstated: the graph keeps no
// initial node for the runtime to report, since the lowerer cannot choose.
func TestStateBehaviorBodyWithAmbiguousStartKeepsNoInitial(t *testing.T) {
	for name, body := range map[string]string{
		"two unpreceded steps": `action a : A; action b : A; action c : A; succession first a then c; succession first b then c;`,
		"a cycle":              `action a : A; action b : A; succession first a then b; succession first b then a;`,
	} {
		graph, err := ToStateGraph(stateUsageIn(t, `
			package test {
				action def A;
				state Machine {
					entry; then s;
					state s { do action ops { `+body+` } }
					succession first s then done;
				}
			}
		`), nil)
		if err != nil {
			t.Fatalf("%s: ToStateGraph: %v", name, err)
		}
		block, ok := graph.Behaviors[stateNamed(graph, "s")].Do[0].Body[0].(Block)
		if !ok || block.Graph == nil {
			t.Fatalf("%s: do body is not a stated flow", name)
		}
		if block.Graph.Initial != nil {
			t.Errorf("%s: initial node = %v, want none", name, block.Graph.Initial)
		}
	}
}
