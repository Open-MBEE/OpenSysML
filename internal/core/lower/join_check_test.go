package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// joinMachine wraps a parallel state `work` with regions left and right, whose
// bodies and the transitions into `sync` the caller writes.
func joinMachine(left, right, segments string) string {
	return `
		package test {
			attribute def Go;
			attribute def Stop;
			state def Machine {
				entry; then work;
				state work parallel {
					state left { ` + left + ` }
					state right { ` + right + ` }
				}
				state rest;
				join sync;
				` + segments + `
				transition first sync then rest;
			}
		}
	`
}

// A join's incoming transitions leave one state per region: a second segment out
// of one source, or out of another state of its region, is refused when lowered,
// whatever its trigger or guard.
func TestToStateGraph_JoinSegmentsShareRegionFail(t *testing.T) {
	for name, tc := range map[string]struct{ segments, want string }{
		"two segments from one source": {
			`transition first a accept Go then sync;
			 transition first a accept Stop then sync;
			 transition first b then sync;`,
			"two incoming transitions leave a",
		},
		"two sources in one region": {
			`transition first a then sync;
			 transition first a2 if true then sync;
			 transition first b then sync;`,
			"leave a and a2, in the same region",
		},
		"source below a composite of the region": {
			`transition first a then sync;
			 transition first w0 then sync;
			 transition first b then sync;`,
			"leave a and w0, in the same region",
		},
		"source outside the regions": {
			`transition first a then sync;
			 transition first b then sync;
			 transition first rest then sync;`,
			"leaves rest, which is not in an orthogonal region",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ToStateGraph(stateDefinitionIn(t, joinMachine(
				`entry; then a; state a; state a2;
				 state wrapper { entry; then w0; state w0; }`,
				`entry; then b; state b;`,
				tc.segments,
			)), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

// One segment per region is the join's shape, the source lying in the region
// directly or below a composite state of it; the plan names the state the
// sources all lie below and the region of it each segment leaves.
func TestToStateGraph_JoinSegmentsOnePerRegion(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, joinMachine(
		`entry; then wrapper; state wrapper { entry; then w0; state w0; }`,
		`entry; then b; state b;`,
		`transition first w0 then sync;
		 transition first b then sync;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	var plan *JoinPlan
	for join, p := range graph.JoinPlans {
		if join.Name == "sync" {
			plan = p
		}
	}
	if plan == nil {
		t.Fatal("no plan recorded for join sync")
	}
	if plan.Owner == nil || plan.Owner.Name != "work" {
		t.Fatalf("plan owner = %v, want work", plan.Owner)
	}
	regions := make(map[string]string, len(plan.Regions))
	for trans, region := range plan.Regions {
		regions[vertexName(trans.Source)] = region.Name
	}
	want := map[string]string{"w0": "left", "b": "right"}
	if len(regions) != len(want) {
		t.Fatalf("plan regions = %v, want %v", regions, want)
	}
	for source, region := range want {
		if regions[source] != region {
			t.Errorf("segment out of %s leaves region %q, want %q", source, regions[source], region)
		}
	}
}

// parallelMachine is a machine whose own regions are left and right, with a
// nested orthogonal state inner in left; segments names the transitions into sync.
func parallelMachine(segments string) string {
	return `
		package test {
			attribute def Go;
			state def Machine parallel {
				state left {
					entry; then inner;
					state inner parallel {
						state l1 { entry; then a; state a; }
						state l2 { entry; then c; state c; }
					}
				}
				state right {
					entry; then b;
					state b;
				}
				join sync;
				` + segments + `
				transition first sync then done;
			}
		}
	`
}

// A join of the machine's own regions records the top-level region each segment
// leaves, however deep its source lies, so the region is left once and whole.
func TestToStateGraph_JoinOfMachineRegionsRecordsTopLevelRegions(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, parallelMachine(
		`transition first a accept Go then sync;
		 transition first b accept Go then sync;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	var plan *JoinPlan
	for join, p := range graph.JoinPlans {
		if join.Name == "sync" {
			plan = p
		}
	}
	if plan == nil {
		t.Fatal("no plan recorded for join sync")
	}
	if plan.Owner != nil {
		t.Fatalf("plan owner = %s, want none: the machine's own regions are joined", plan.Owner.Name)
	}
	regions := make(map[string]*ast.StateRegion, len(plan.Regions))
	for trans, region := range plan.Regions {
		regions[vertexName(trans.Source)] = region
	}
	for source, want := range map[string]int{"a": 0, "b": 1} {
		if regions[source] != graph.TopRegions[want] {
			t.Errorf("segment out of %s leaves region %q, want the machine's region %q", source, regions[source].Name, graph.TopRegions[want].Name)
		}
	}
}

// Two segments out of one of the machine's regions are refused however the
// sources are nested, and so are sources in regions of different orthogonal states.
func TestToStateGraph_JoinOfMachineRegionsShareRegionFail(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"nested sources in one top-level region": {
			parallelMachine(`transition first a accept Go then sync;
			 transition first c accept Go then sync;
			 transition first b accept Go then sync;`),
			"leave a and c, in the same region",
		},
		"sources in regions of two composite states": {
			`package test {
				attribute def Go;
				state def Machine {
					entry; then w1;
					state w1 parallel {
						state l { entry; then a; state a; }
						state r { entry; then b; state b; }
					}
					state w2 parallel {
						state l { entry; then c; state c; }
						state r { entry; then d; state d; }
					}
					join sync;
					transition first a accept Go then sync;
					transition first c accept Go then sync;
					transition first sync then done;
				}
			}`,
			"leave regions of more than one orthogonal state",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ToStateGraph(stateDefinitionIn(t, tc.src), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
