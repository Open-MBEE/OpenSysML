package model

import (
	"slices"
	"testing"
)

// A calc declaring no return has only `Evaluation::result`, typed Anything, which
// says nothing of what its body yields: an invocation of it is of no known type.
func TestUndeclaredCalcResultIsOfNoKnownType(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("test.sysml", []byte(`package P {
		private import ScalarValues::*;
		calc def Ready { in n : Integer; n >= 1 }
		calc def Next { in n : Integer; n + 1 }
		calc def Wrap { in n : Integer; Ready(n) }
		part def W {
			attribute alive : Boolean = true;
			attribute uses : Integer = 1;
			attribute ok : Boolean = alive and Ready(uses);
			attribute negated : Boolean = not Ready(uses);
			attribute bound : Boolean = Ready(uses);
			attribute after : Integer = Next(uses) + 1;
			attribute chosen : Integer = if Ready(uses) ? 1 else 0;
			attribute kept = Ready(uses);
			attribute both : Boolean = alive and kept;
			attribute neither : Boolean = not kept;
			attribute wrapped : Boolean = alive and Wrap(uses);
			constraint c { alive and Ready(uses) }
			state day {
				entry; then a;
				state a;
				transition first a if alive and Ready(uses) then a;
			}
		}
	}`), 1)
	if found := exprTypeDiagnostics(ws, "test.sysml"); len(found) != 0 {
		t.Fatalf("expression type findings = %v, want none", found)
	}
}

// A constraint declaring no return still yields a Boolean: its kind base's result
// (`BooleanEvaluation::result`) is typed, unlike a calc's.
func TestUndeclaredConstraintResultIsBoolean(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("test.sysml", []byte(`package P {
		private import ScalarValues::*;
		constraint def Ready { in n : Integer; n >= 1 }
		part def W {
			attribute uses : Integer = 1;
			attribute ok : Boolean = Ready(uses) and true;
			attribute text = Ready(uses) as String;
			attribute count : Integer = Ready(uses);
		}
	}`), 1)
	var codes []string
	for _, d := range ws.Diagnostics("test.sysml") {
		codes = append(codes, d.Code)
	}
	for _, want := range []string{"cast-conformance", "binding-connector-type-conformance"} {
		if !slices.Contains(codes, want) {
			t.Errorf("findings = %v, want %s among them", codes, want)
		}
	}
	if slices.Contains(codes, "operator-operand-type") {
		t.Errorf("findings = %v, want no operand finding on the Boolean result", codes)
	}
}
