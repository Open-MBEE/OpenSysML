package solve

import (
	"errors"
	"strings"
	"testing"
)

// The container's check ranges over the values bound to a subrequirement, which the solver does not
// enumerate: it refuses rather than encode the declaration's conditions for a binding that may yield none.
func TestBoundSubrequirementIsRefused(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			requirement def Bad { subject x : Real; require constraint { x > 1.0 } }
			requirement def R { subject x : Real; requirement s[0..1] : Bad = (); }
		}
	`
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, "test::R")
	var refused *NotTranslatableError
	if q, err := Requirement(ctx, sym, sym.OwnerScope); !errors.As(err, &refused) {
		t.Fatalf("translating R: %v, %v; want a refusal", q, err)
	}
	if !strings.Contains(refused.Construct, "bound subrequirement s") || !strings.Contains(refused.Error(), "not covered") {
		t.Errorf("refusal = %v, want the bound subrequirement named as not covered", refused)
	}
	if at := src[refused.Span.Offset:refused.Span.End()]; at != "()" {
		t.Errorf("refusal is located at %q, want the bound value ()", at)
	}
}
