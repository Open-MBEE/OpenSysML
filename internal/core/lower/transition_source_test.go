package lower

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A transition written without a source leaves the state declared before it in
// the same body (SysML v2 §7.18.3): several in a row all leave that state, a
// succession between them is looked past, and the rule holds in a composite
// state's body as it does at the machine's top level.
func TestToStateGraph_SourcelessTransitionLeavesThePrecedingState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				then state normal;
				accept Go then maintenance;
				if ready then degraded;
				state maintenance;
				accept after 5 [s] then normal;
				state degraded {
					entry; then low;
					state low;
					accept Go then high;
					state high;
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	wantLeaves := map[string][]string{
		"start":       {"normal"},
		"normal":      {"maintenance", "degraded"},
		"maintenance": {"normal"},
		"low":         {"high"},
		"high":        nil,
		"degraded":    nil,
	}
	for name, targets := range wantLeaves {
		state := stateNamed(graph, name)
		if state == nil {
			t.Fatalf("no state %s in the graph", name)
		}
		got := graph.Transitions[state]
		if len(got) != len(targets) {
			t.Fatalf("%s leaves %d transitions, want %d (%v)", name, len(got), len(targets), targets)
		}
		for i, target := range targets {
			if reached, ok := got[i].Target.(*ast.StateNode); !ok || reached.Name != target {
				t.Errorf("%s transition %d reaches %v, want %s", name, i, got[i].Target, target)
			}
		}
	}
}

// A definition's shorthand leaves the state before it in the definition's body,
// materialized once per usage typed by it, and a usage's own shorthand looks back
// through the usage's body alone, as the pilot scans only the owning type's
// memberships: written first in the usage it has nothing to leave.
func TestToStateGraph_InheritedSourcelessTransitionLeavesEachMaterialization(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Inner {
				entry; then a;
				state a;
				accept Go then b;
				state b;
			}
			state def Machine {
				entry; then one;
				state one : Inner;
				state two : Inner;
			}
		}
	`, "Machine")

	var leaves []*Transition
	for source, transitions := range graph.Transitions {
		if state, ok := source.(*ast.StateNode); ok && state.Name == "a" {
			leaves = append(leaves, transitions...)
		}
	}
	if len(leaves) != 2 {
		t.Fatalf("the inherited shorthand lowered into %d transitions, want one per usage", len(leaves))
	}
	if leaves[0].Source == leaves[1].Source || leaves[0].Target == leaves[1].Target {
		t.Fatal("the two usages share a vertex")
	}
	for _, trans := range leaves {
		source, target := trans.Source.(*ast.StateNode), trans.Target.(*ast.StateNode)
		if target.Name != "b" {
			t.Errorf("transition reaches %s, want b", target.Name)
		}
		if graph.ParentState[source] != graph.ParentState[target] {
			t.Errorf("a in %v leaves for b in %v", graph.ParentState[source], graph.ParentState[target])
		}
	}

	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state def Inner {
				entry; then a;
				state a;
				state b;
			}
			state Machine : Inner {
				accept Go then b;
			}
		}
	`), nil)
	if !errors.Is(err, ErrNoTransitionSource) {
		t.Fatalf("got %v, want ErrNoTransitionSource", err)
	}
}

// As the first member of its body the shorthand has nothing before it to leave.
func TestToStateGraph_SourcelessTransitionWithNothingBefore(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				accept Go then active;
				entry; then init;
				state init;
				state active;
			}
		}
	`), nil)
	if !errors.Is(err, ErrNoTransitionSource) {
		t.Fatalf("got %v, want ErrNoTransitionSource", err)
	}
}

// `entry; if c then s;` is a guarded entry transition, whose starting-state
// choice the lowering reports as unsupported rather than reading a wrong source.
func TestToStateGraph_GuardedEntryTransitionIsUnsupported(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				attribute cold : Boolean = true;
				entry;
				if cold then warming;
				state warming;
			}
		}
	`), nil)
	if !errors.Is(err, ErrEntryTransitionUnsupported) {
		t.Fatalf("got %v, want ErrEntryTransitionUnsupported", err)
	}
}

// The member before the shorthand is named in the error when it is not a
// vertex, whatever kind of member it is.
func TestToStateGraph_SourcelessTransitionAfterANonVertex(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"entry action": {
			body: `entry; then init;
				accept Go then active;
				state init;
				state active;`,
			want: "the entry action",
		},
		"start marker": {
			body: `entry; then init;
				state init;
				first init then active;
				accept Go then done;
				state active;`,
			want: "the succession from init",
		},
		"explicit transition": {
			body: `entry; then init;
				state init;
				transition first init accept Go then active;
				accept Stop then done;
				state active;`,
			want: "the transition",
		},
		"succession usage": {
			body: `entry; then init;
				state init;
				state active;
				succession first init then active;
				accept Go then done;`,
			want: "an unnamed succession usage",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ToStateGraph(stateUsageIn(t, `
				package test {
					state Machine {
						`+tc.body+`
					}
				}
			`), nil)
			var sourceErr *TransitionSourceError
			if !errors.As(err, &sourceErr) {
				t.Fatalf("got %v, want a TransitionSourceError", err)
			}
			if sourceErr.Region {
				t.Errorf("%v reports an orthogonal region", err)
			}
			if got := err.Error(); !strings.Contains(got, "transition without a source leaves "+tc.want+", the member declared before it") {
				t.Errorf("got %q, want it to name %s", got, tc.want)
			}
		})
	}
}

// The region of a parallel state before the shorthand is not a vertex of the
// machine: the error says so, rather than naming it as any other member.
func TestToStateGraph_SourcelessTransitionAfterARegion(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then p;
				state p parallel {
					state r1 {
						entry; then a;
						state a;
					}
					accept Go then r2.b;
					state r2 {
						entry; then b;
						state b;
					}
				}
			}
		}
	`), nil)
	var sourceErr *TransitionSourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("got %v, want a TransitionSourceError", err)
	}
	if !sourceErr.Region {
		t.Errorf("%v does not report an orthogonal region", err)
	}
	if want := "leaves the state usage r1, the member declared before it, which is an orthogonal region"; !strings.Contains(err.Error(), want) {
		t.Errorf("got %q, want it to contain %q", err, want)
	}
}
