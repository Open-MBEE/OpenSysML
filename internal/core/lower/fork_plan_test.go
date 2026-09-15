package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// forkMachine wraps a parallel state `work` with regions left and right, whose
// bodies and the transitions out of `split` the caller writes.
func forkMachine(left, right, branches string) string {
	return `
		package test {
			state def Machine {
				entry; then idle;
				state idle;
				state work parallel {
					state left { ` + left + ` }
					state right { ` + right + ` }
				}
				fork split;
				transition first idle then split;
				` + branches + `
			}
		}
	`
}

func forkNamed(graph *StateGraph, name string) *ast.PseudostateNode {
	for _, ps := range graph.Pseudostates {
		if ps.Name == name {
			return ps
		}
	}
	return nil
}

// A region only a fork enters needs no entry transition: the fork's branch is
// recorded as the region's way in, and the branch keeps its effect.
func TestToStateGraph_ForkEntersRegionsWithoutInitial(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, forkMachine(
		`state a;`,
		`state b;`,
		`transition first split do { assign x := 1; } then a;
		 transition first split then b;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	work := stateNamed(graph, "work")
	regions := graph.CompositeStates[work]
	if len(regions) != 2 {
		t.Fatalf("work has %d regions, want 2", len(regions))
	}
	plan := graph.ForkPlans[forkNamed(graph, "split")]
	if plan == nil {
		t.Fatal("fork split has no plan")
	}
	if plan.Owner != work {
		t.Fatalf("plan owner = %v, want work", plan.Owner)
	}
	for _, region := range regions {
		if len(graph.EntryTransitions[region]) != 0 {
			t.Errorf("region %s has entry transitions, want none", region.Name)
		}
		if graph.RegionInitials[region] != nil {
			t.Errorf("region %s has an initial %s, want none", region.Name, graph.RegionInitials[region].Name)
		}
		if !graph.ForkStarted(region) {
			t.Errorf("region %s is not fork-started", region.Name)
		}
		branch := plan.Branches[region]
		if branch == nil {
			t.Fatalf("region %s has no branch in the plan", region.Name)
		}
		if graph.RegionOf[branch.Target.(*ast.StateNode)] != region {
			t.Errorf("branch into %s targets %s outside it", region.Name, getNodeName(branch.Target))
		}
	}
	targets := plan.Targets()
	a, b := stateNamed(graph, "a"), stateNamed(graph, "b")
	if targets[graph.RegionOf[a]] != a || targets[graph.RegionOf[b]] != b {
		t.Errorf("targets = %v, want a and b in their regions", targets)
	}
	if len(plan.Branches[graph.RegionOf[a]].Effect) != 1 {
		t.Errorf("branch into a lost its effect")
	}
}

// A region with an entry transition of its own keeps it even when a fork also
// enters it: the two ways in are recorded apart.
func TestToStateGraph_ForkKeepsRegionInitialApart(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, forkMachine(
		`entry; then a0; state a0; state a;`,
		`state b;`,
		`transition first split then a;
		 transition first split then b;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	a0 := stateNamed(graph, "a0")
	left := graph.RegionOf[a0]
	if graph.RegionInitials[left] != a0 {
		t.Errorf("left starts at %v, want a0", graph.RegionInitials[left])
	}
	if got := graph.ForkPlans[forkNamed(graph, "split")].Targets()[left]; got != stateNamed(graph, "a") {
		t.Errorf("fork enters left at %v, want a", got)
	}
}

// A region with neither an entry transition nor a fork branch has no way in.
func TestToStateGraph_RegionWithoutInitialOrForkFails(t *testing.T) {
	_, err := ToStateGraph(stateDefinitionIn(t, `
		package test {
			state def Machine {
				entry; then idle;
				state idle;
				state work parallel {
					state left { state a; }
					state right { state b; }
					state third { state c; }
				}
				fork split;
				transition first idle then split;
				transition first split then a;
				transition first split then b;
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("region no fork enters succeeded")
	}
	if !strings.Contains(err.Error(), "region third has no initial state") {
		t.Fatalf("error = %q, want missing initial for third", err)
	}
}

// The fork's own shape is checked as the graph is built.
func TestToStateGraph_ForkShapeRejected(t *testing.T) {
	for name, tc := range map[string]struct{ branches, want string }{
		"guarded branch": {
			`transition first split if true then a;
			 transition first split then b;`,
			"cannot be guarded",
		},
		"two branches into one region": {
			`transition first split then a;
			 transition first split then a2;
			 transition first split then b;`,
			"in the same region",
		},
		"one branch": {
			`transition first split then a;`,
			"needs at least two outgoing transitions",
		},
		"branch outside a region": {
			`transition first split then a;
			 transition first split then idle;`,
			"not in an orthogonal region",
		},
		"branch into a pseudostate": {
			`transition first split then a;
			 transition first split then sync;`,
			"branch target must be a state",
		},
		"branches into two composite states": {
			`transition first split then a;
			 transition first split then c;`,
			"span more than one composite state",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ToStateGraph(stateDefinitionIn(t, forkMachine(
				`state a; state a2;`,
				`state b;`,
				tc.branches+`
				 join sync;
				 transition first a then sync;
				 transition first b then sync;
				 transition first sync then done;
				 state other parallel {
				 	state third { entry; then c; state c; }
				 	state fourth { entry; then d; state d; }
				 }`,
			)), nil)
			if err == nil {
				t.Fatal("malformed fork succeeded")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
		})
	}
}
