package runtime

import (
	"strings"
	"testing"
)

// TestChainOverCollectionTraceOrder pins the order a chain through multi-valued
// features reads its objects in: declaration order, depth first, one flattening
// per step.
func TestChainOverCollectionTraceOrder(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Leaf { attribute v : Real; }
			part def Mid { part leaves : Leaf[*]; }
			part def Top {
				part mids : Mid[*];
				attribute values : Real[*] = mids.leaves.v;
			}
			part top : Top {
				part m1 : Mid :> mids {
					part l1 : Leaf :> leaves { attribute :>> v = 1.0; }
					part l2 : Leaf :> leaves { attribute :>> v = 2.0; }
				}
				part m2 : Mid :> mids {
					part l3 : Leaf :> leaves { attribute :>> v = 4.0; }
				}
			}
		}
	`))
	matches := idx.LookupQualified("test::top")
	if len(matches) != 1 {
		t.Fatalf("test::top: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	fv, err := inst.GetFeatureValue(ctx, "values")
	if err != nil {
		t.Fatalf("GetFeatureValue(values): %v", err)
	}
	if got := FormatTraceValue(fv.HeldValue()); got != "(1.0, 2.0, 4.0)" {
		t.Fatalf("values = %s, want (1.0, 2.0, 4.0)", got)
	}

	// Each step of the chain answers the collection of the step before it,
	// flattened once, in the order the objects it read are declared in, and the
	// objects a step materializes are reported where they are materialized.
	want := strings.Join([]string{
		"materialize: m1 #2",
		"materialize: m2 #3",
		"    eval feature mids -> (instance#2, instance#3)",
		"materialize: l1 #4",
		"materialize: l2 #5",
		"materialize: l3 #6",
		"  eval chain leaves -> (instance#4, instance#5, instance#6)",
		"eval chain v -> (1.0, 2.0, 4.0)",
	}, "\n")
	if got := trace.String(); got != want {
		t.Errorf("trace =\n%s\nwant\n%s", got, want)
	}
}
