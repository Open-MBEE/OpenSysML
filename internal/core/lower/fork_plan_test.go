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

// A region only a fork enters must be entered only by that fork: any other way
// into its composite state would start the region by default, which it cannot.
func TestToStateGraph_ForkOnlyRegionEnteredByDefaultFails(t *testing.T) {
	for name, tc := range map[string]struct{ entry, extra, want string }{
		"transition into the composite state": {
			`entry; then idle;`,
			`transition first idle accept Go then work;`,
			"the transition from idle to work enters work",
		},
		"transition into the other region": {
			`entry; then idle;`,
			`transition first idle accept Go then b;`,
			"the transition from idle to b enters work",
		},
		"self-transition of the composite state": {
			`entry; then idle;`,
			`transition first work accept Go then work;`,
			"the transition from work to work enters work",
		},
		"transition into the composite state's history": {
			`entry; then idle;`,
			`transition first idle accept Go then back;`,
			"the transition from idle to back enters work",
		},
		"entry transition naming the composite state": {
			`entry; then work;`,
			``,
			"the entry transition naming work",
		},
		"entry transition naming a state in the other region": {
			`entry; then b;`,
			``,
			"the entry transition naming b",
		},
		"guarded entry transition naming a state in the other region": {
			`entry; if false then idle; then b;`,
			``,
			"the entry transition naming b",
		},
		"junction outside leading into the other region": {
			`entry; then idle;`,
			`junction j; transition first idle accept Go then j; transition first j then b;`,
			"the transition from idle to b enters work",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ToStateGraph(stateDefinitionIn(t, `
				package test {
					attribute def Go;
					state def Machine {
						`+tc.entry+`
						state idle;
						state work parallel {
							state left { state a; }
							state right { state b; }
							history back;
						}
						fork split;
						transition first idle then split;
						transition first split then a;
						transition first split then b;
						`+tc.extra+`
					}
				}
			`), nil)
			if err == nil {
				t.Fatal("default entry into a fork-only region succeeded")
			}
			if !strings.Contains(err.Error(), "region left in state work has no initial state") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
		})
	}
}

// An enclosing state's entry transition naming a state in the other region enters
// the composite state from outside it too; the other region's own is fine.
func TestToStateGraph_ForkOnlyRegionEnteredByAnEnclosingEntryFails(t *testing.T) {
	machine := func(outerEntry, rightEntry string) string {
		return `
			package test {
				attribute def Go;
				state def Machine {
					entry; then idle;
					state idle;
					state outer {
						` + outerEntry + `
						state start;
						state work parallel {
							state left { state a; }
							state right { ` + rightEntry + ` state b; }
						}
						fork split;
						transition first start accept Go then split;
						transition first split then a;
						transition first split then b;
					}
					transition first idle accept Go then outer;
				}
			}
		`
	}
	_, err := ToStateGraph(stateDefinitionIn(t, machine(`entry; then b;`, ``)), nil)
	if err == nil || !strings.Contains(err.Error(), "the entry transition naming b") {
		t.Fatalf("outer's entry naming b: error = %v, want the entry transition naming b", err)
	}
	if _, err := ToStateGraph(stateDefinitionIn(t, machine(`entry; then start;`, `entry; then b;`)), nil); err != nil {
		t.Fatalf("right's own entry naming b: %v", err)
	}
}

// A fork that omits a region starts it by default, so a region another fork
// alone enters is refused; with an entry transition the omission is fine.
func TestToStateGraph_ForkOnlyRegionOmittedByAnotherForkFails(t *testing.T) {
	machine := func(third string) string {
		return `
			package test {
				attribute def Go;
				state def Machine {
					entry; then idle;
					state idle;
					state work parallel {
						state left { state a; }
						state right { state b; }
						state third { ` + third + ` state c; }
					}
					fork f1;
					fork f2;
					transition first idle accept Go then f1;
					transition first idle then f2;
					transition first f1 then a;
					transition first f1 then b;
					transition first f2 then b;
					transition first f2 then c;
				}
			}
		`
	}
	_, err := ToStateGraph(stateDefinitionIn(t, machine(``)), nil)
	if err == nil || !strings.Contains(err.Error(), "region left in state work has no initial state") || !strings.Contains(err.Error(), "fork f2 enters work") {
		t.Fatalf("f2 omitting left: error = %v, want fork f2 enters work", err)
	}
	_, err = ToStateGraph(stateDefinitionIn(t, machine(`entry; then c;`)), nil)
	if err == nil || !strings.Contains(err.Error(), "region left in state work has no initial state") {
		t.Fatalf("third with its own entry: error = %v, want left still refused", err)
	}
}

// A fork's branches enter every composite state above their targets, so a fork
// into a nested composite state starts an outer sibling region by default, and a
// fork above a composite state whose branch names it starts the regions of that
// state the nearer fork alone enters. A fork reached from inside the outer state
// leaves its other region as it is.
func TestToStateGraph_ForkOnlyRegionEnteredByANestedForkFails(t *testing.T) {
	machine := func(o2Entry, leftEntry, routes string) string {
		return `
			package test {
				attribute def Go;
				state def Machine {
					entry; then idle;
					state idle;
					state outer parallel {
						state o1 {
							entry; then hold;
							state hold;
							state inner parallel {
								state left { ` + leftEntry + ` state a; }
								state right { state b; }
							}
							fork split;
							transition first hold accept Go then split;
							transition first split then a;
							transition first split then b;
						}
						state o2 { ` + o2Entry + ` state c; }
					}
					fork split2;
					transition first split2 then c;
					` + routes + `
				}
			}
		`
	}
	_, err := ToStateGraph(stateDefinitionIn(t, machine(``, ``,
		`transition first idle then split;
		 transition first split2 then hold;
		 transition first idle accept Go then split2;`)), nil)
	if err == nil || !strings.Contains(err.Error(), "region o2 in state outer has no initial state") || !strings.Contains(err.Error(), "the transition from idle to split enters outer") {
		t.Fatalf("nested fork from outside: error = %v, want the transition from idle to split", err)
	}
	_, err = ToStateGraph(stateDefinitionIn(t, machine(``, `entry; then a;`,
		`transition first idle then split2;
		 transition first split2 then inner;`)), nil)
	if err == nil || !strings.Contains(err.Error(), "region right in state inner has no initial state") || !strings.Contains(err.Error(), "the transition from idle to split2 enters inner") {
		t.Fatalf("outer fork naming inner: error = %v, want the transition from idle to split2", err)
	}
	if _, err := ToStateGraph(stateDefinitionIn(t, machine(``, ``,
		`transition first idle then split2;
		 transition first split2 then hold;`)), nil); err != nil {
		t.Fatalf("nested fork reached from inside outer: %v", err)
	}
}

// A transition that names a state inside the fork-only region, or one that stays
// inside the composite state — directly or through a junction declared there —
// starts no region by default and is accepted.
func TestToStateGraph_ForkOnlyRegionKeepsExplicitEntries(t *testing.T) {
	_, err := ToStateGraph(stateDefinitionIn(t, forkMachine(
		`state a; state a2; transition first a accept Go then a2;`,
		`state b; state b2; entry; then b; junction j;
		 transition first b accept Go then j; transition first j then b2;`,
		`transition first split then a;
		 transition first split then b;
		 transition first idle accept Go then a2;
		 transition first b2 accept Go then a;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
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
		"triggered branch": {
			`transition first split accept Go then a;
			 transition first split then b;`,
			"cannot have triggers",
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
				 attribute def Go;
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
