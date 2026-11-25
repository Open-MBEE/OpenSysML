package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessSubsettingElsewhere: a feature subsetting a feature of another object
// contributes to it, so what goes wrong on the way is the typed error of the read, never a
// silent omission, a panic or unbounded recursion.
func TestRuntimeRobustnessSubsettingElsewhere(t *testing.T) {
	const prelude = `
		private import ScalarValues::Integer;
		part def Sub { attribute k : Integer default 1; }
	`
	cases := []struct {
		name, model, path string
		want              error
	}{
		{"more_nested_subsetters_than_the_multiplicity_admits", `
			part def One { part only : Sub[0..1]; part frame { part a : Sub :> only; part b : Sub :> only; } }
			part root : One;`, "only", ErrMultiplicityViolation},
		{"nested_subsetter_valued_by_what_it_subsets", `
			part def Cyc { part xs : Sub[*]; part frame { ref part ys : Sub[*] :> xs = xs; } }
			part root : Cyc;`, "xs", ErrCyclicFeatureValue},
		{"usage_leading_to_the_subsetter_cannot_be_read", `
			part def Bad { part xs : Sub[*]; ref part frame : Holder = 1 / 0; }
			part def Holder { part y : Sub; }
			part def Worse :> Bad { part :>> frame { part z : Sub :> xs; } }
			part root : Worse;`, "xs", ErrDivisionByZero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, inst, err := instantiateWithLibraries(t, "package test {"+prelude+tc.model+"\n}", "test::root")
			if err != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("instantiate = %v, want %v", err, tc.want)
				}
				return
			}
			if _, err = featureValueAtPath(t, ctx, inst, tc.path); !errors.Is(err, tc.want) {
				t.Fatalf("%s = %v, want %v", tc.path, err, tc.want)
			}
		})
	}
}
