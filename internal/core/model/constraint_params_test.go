package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// A constraint that declares its parameters still states bare conditions: the
// parameters decide which body parser reads the body, and both read conditions.
func TestConstraintWithParametersStatesConditions(t *testing.T) {
	const src = `package P {
    private import ScalarValues::*;

    constraint validRange {
        in x : Real;
        in initialized : Boolean;

        x >= 0
        not (x > 100)
        initialized
    }
}`

	ws := NewWorkspace()
	ws.Open("t.sysml", []byte(src), 1)

	var errs []string
	for _, d := range ws.Diagnostics("t.sysml") {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	if len(errs) > 0 {
		t.Errorf("unexpected error(s): %s", strings.Join(errs, "; "))
	}
}

// A body expression's parameter is typed, so a feature of its type is reachable
// through it: `{p : Point; p.x > 0}` resolves `x` in Point.
func TestBodyExprParameterMembersResolve(t *testing.T) {
	const src = `package P {
    private import ScalarValues::*;
    private import ControlFunctions::exists;

    classifier Point { feature x: Real; }

    feature Shape {
        feature vertices: Point[*];
        feature hasPositive: Boolean = vertices->exists{p : Point; p.x > 0};
    }
}`

	ws := NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)

	var errs []string
	for _, d := range ws.Diagnostics("t.kerml") {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	if len(errs) > 0 {
		t.Errorf("unexpected error(s): %s", strings.Join(errs, "; "))
	}
}
