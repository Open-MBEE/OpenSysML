package lower

import (
	"strings"
	"testing"
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
// directly or below a composite state of it.
func TestToStateGraph_JoinSegmentsOnePerRegion(t *testing.T) {
	_, err := ToStateGraph(stateDefinitionIn(t, joinMachine(
		`entry; then wrapper; state wrapper { entry; then w0; state w0; }`,
		`entry; then b; state b;`,
		`transition first w0 then sync;
		 transition first b then sync;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
}
