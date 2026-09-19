package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// F66: the declaration an `assume`/`require constraint` owns resolves like any
// other, down to the bounds of its multiplicity.
func TestF66OwnedConstraintDeclarationResolves(t *testing.T) {
	const src = `package RequirementTest {
    attribute n = 2;
    constraint def C;
    requirement def R {
        assume constraint c1 : C[n];
        require constraint c2 : C[1..n];
    }
}`

	ws := NewWorkspace()
	ws.Open("f66.sysml", []byte(src), 1)

	var errs []string
	for _, d := range ws.Diagnostics("f66.sysml") {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	if len(errs) > 0 {
		t.Fatalf("expected the model to analyse cleanly, got:\n%s", strings.Join(errs, "\n"))
	}
}

// An undefined bound in an owned constraint's multiplicity must be diagnosed:
// the bounds are resolved, not skipped.
func TestF66OwnedConstraintMultiplicityBoundIsResolved(t *testing.T) {
	const src = `package RequirementTest {
    constraint def C;
    requirement def R {
        assume constraint c1 : C[missingBound];
    }
}`

	ws := NewWorkspace()
	ws.Open("f66_bad.sysml", []byte(src), 1)

	var found bool
	for _, d := range ws.Diagnostics("f66_bad.sysml") {
		if d.Severity == diag.SeverityError && strings.Contains(d.Message, "missingBound") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an unresolved-reference error for the multiplicity bound")
	}
}
