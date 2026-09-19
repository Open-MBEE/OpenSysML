package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// requirePrelude declares the requirement a require/assume body refers to.
const requirePrelude = `package Q {
	requirement def R { attribute mass : ScalarValues::Real; }
	requirement r : R;
}
`

func requireBodyDiags(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	return typeDiags(t, scalarPrelude+valuePrelude+requirePrelude+`package P {
		analysis def C {
			objective o {
				require Q::r {`+body+`}
			}
		}
	}`)
}

const mismatch = "cannot bind String value to a feature typed by Real"

// A require body is a scope of its own, so the type checker must read the names
// it declares there rather than in the enclosing declaration.
func TestTypecheckReadsARequireBodyLocalName(t *testing.T) {
	diags := requireBodyDiags(t, `
		attribute local : ScalarValues::String;
		attribute bad : ScalarValues::Real = local;
	`)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic for a body-local mismatch, got %v", diags)
	}
	if got := diags[0].Message; got != mismatch {
		t.Fatalf("message %q, want %q", got, mismatch)
	}
}

// A legal body-local value stays clean, and a body nested inside it is typed in
// its own scope too.
func TestTypecheckRequireBodyLocalValuesStayClean(t *testing.T) {
	if diags := requireBodyDiags(t, `
		attribute local : ScalarValues::Real;
		attribute reads : ScalarValues::Real = local;
		part p : M::Vehicle {
			attribute deep : ScalarValues::String;
			attribute readsDeep : ScalarValues::String = deep;
		}
	`); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// A mismatch two levels inside the body is reported: the walk carries the body
// scope down rather than stopping at its direct members.
func TestTypecheckReportsAMismatchNestedInARequireBody(t *testing.T) {
	diags := requireBodyDiags(t, `
		part p : M::Vehicle {
			attribute deep : ScalarValues::String;
			attribute bad : ScalarValues::Real = deep;
		}
	`)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic at depth two, got %v", diags)
	}
	if got := diags[0].Message; got != mismatch {
		t.Fatalf("message %q, want %q", got, mismatch)
	}
}

// A name written in the body is typed against the body's own declaration, not
// an unrelated one of the same name in the enclosing scope.
func TestTypecheckRequireBodyLocalNameShadowsTheEnclosingOne(t *testing.T) {
	diags := typeDiags(t, scalarPrelude+valuePrelude+requirePrelude+`package P {
		analysis def C {
			attribute shared : ScalarValues::Real;
			objective o {
				require Q::r {
					attribute shared : ScalarValues::String;
					attribute bad : ScalarValues::Real = shared;
				}
			}
		}
	}`)
	if len(diags) != 1 {
		t.Fatalf("expected the body's own 'shared' to be typed, got %v", diags)
	}
	if got := diags[0].Message; got != mismatch {
		t.Fatalf("message %q, want %q", got, mismatch)
	}
}
