package runtime

import (
	"errors"
	"fmt"
	"testing"
)

// satisfyReferenceModel is a satisfaction in the reference form; the satisfy is substituted per case.
const satisfyReferenceModel = `package test {
	private import ScalarValues::*;
	part def Relay { attribute rating : Real; }
	requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
	requirement ratingOk : RatingReq;
	requirement group { requirement nested : RatingReq; }
	part relay : Relay;
	%s
}`

// TestRuntimeRobustnessSatisfyReference covers a reference-form satisfy that cannot be judged: the
// requirement it reference-subsets (SysML v2 §8.2.2.21.2), or one subsetted after it, reads a value
// nothing binds. Each is a typed error, never a panic.
func TestRuntimeRobustnessSatisfyReference(t *testing.T) {
	cases := []struct{ name, satisfy string }{
		{"the referenced requirement", `satisfy ratingOk by relay;`},
		{"a requirement reached through a chain", `satisfy group.nested by relay;`},
		{"a requirement subsetted after the reference", `requirement def Free; requirement free : Free; satisfy free :> ratingOk by relay;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(satisfyReferenceModel, tc.satisfy)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
			if len(assertions) != 1 {
				t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
			}
			if _, err := ctx.EvaluateSatisfaction(assertions[0]); !errors.Is(err, ErrNoValue) {
				t.Fatalf("error = %v, want %v", err, ErrNoValue)
			}
		})
	}
}
