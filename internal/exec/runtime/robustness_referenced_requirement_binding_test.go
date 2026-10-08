package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// referencedRequirementModel is a satisfaction whose requirement binds a referenced
// requirement's subject from the reference body; the body is substituted per case.
const referencedRequirementModel = `package test {
	private import ScalarValues::*;
	part def Wagon { attribute tare : Real; attribute cargo : Real; }
	part wagon1 : Wagon {
		attribute :>> tare = 800;
		attribute :>> cargo = 150;
		satisfy wagonSpec by wagon1;
	}
	requirement def WagonMassCap {
		subject w : Wagon;
		attribute actual : Real = w.tare + w.cargo;
		attribute cap : Real;
		require constraint { actual <= cap }
	}
	requirement ladenLimit : WagonMassCap { attribute :>> cap = 1000; }
	requirement wagonSpec { subject unit : Wagon; %s }
}`

// TestRuntimeRobustnessReferencedRequirementBinding covers the failure modes of a
// `require r { in w = unit; }` body: a binding to a name that resolves to nothing, a
// body binding none of the referenced requirement's parameters so its subject stays
// unbound, and a requirement referencing itself. Each is a typed error, never a panic.
func TestRuntimeRobustnessReferencedRequirementBinding(t *testing.T) {
	cases := []struct {
		name, body string
		err        error
		says       string
	}{
		{"a binding to an unresolved name", `require ladenLimit { in w = missing; }`, ErrUnresolvedReference, "missing"},
		{"a body binding a parameter the requirement lacks", `require ladenLimit { in nope = unit; }`, ErrNoValue, "w"},
		{"a reference without a body leaves the subject unbound", `require ladenLimit;`, ErrNoValue, "w"},
		{"a requirement referencing itself", `require wagonSpec { in unit = unit; }`, ErrNoConditions, "no condition"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(referencedRequirementModel, tc.body)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
			if len(assertions) != 1 {
				t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
			}
			_, err := ctx.EvaluateSatisfaction(assertions[0])
			if !errors.Is(err, tc.err) {
				t.Fatalf("error = %v, want %v", err, tc.err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not say %q", err, tc.says)
			}
		})
	}
}
