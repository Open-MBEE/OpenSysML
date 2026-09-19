package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const returnParameterOwnerCode = "return-parameter-owner"

// returnOwnerDiags returns the return-parameter diagnostics of src, asserting
// that each is an error whose span is one of the wanted declarations, in order.
func returnOwnerDiags(t *testing.T, diags []diag.Diagnostic, src string, wantSpans ...string) {
	t.Helper()
	diags = only(diags, returnParameterOwnerCode)
	if len(diags) != len(wantSpans) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(wantSpans), diags)
	}
	for i, d := range diags {
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		if d.Message != msgReturnParameterOwner {
			t.Errorf("message = %q, want %q", d.Message, msgReturnParameterOwner)
		}
		if got := strings.TrimSpace(spanText(src, d)); got != wantSpans[i] {
			t.Errorf("span text = %q, want %q", got, wantSpans[i])
		}
	}
}

// A `return` parameter is owned by a function or expression; an action, part,
// state, port or item owning one is reported, once per declaration, whether the
// owner is a definition, a usage, or a node nested in a calculation.
func TestReturnParameterOutsideFunctionIsReported(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def A { return count : Integer; }
		action a { return count : Integer; }
		part def K { return x : Integer; }
		state def S { return v : Integer; }
		port def PT { return q : Integer; }
		item def I { return r : Integer; }
		calc def C { action inner { return partial : Integer; } return total : Integer; }
	}`
	returnOwnerDiags(t, constraintDiags(t, src), src,
		"return count : Integer;", "return count : Integer;", "return x : Integer;",
		"return v : Integer;", "return q : Integer;", "return r : Integer;", "return partial : Integer;")
}

// Calculations, constraints, cases and their usages are functions or expressions,
// as are requirements, and a body or a redefinition stating the result is silent.
func TestReturnParameterInFunctionIsClean(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		calc def C { in x : Integer; return y : Integer default = x; }
		calc c : C { return :>> y = 3; }
		calc d { in x : Integer; return : Integer = x + 1; }
		constraint def K { in x : Integer; return : Boolean = x > 0; }
		constraint k : K { return :>> result; }
		requirement def R { return : Boolean; }
		analysis def An { return v : Integer; }
		verification def V { return : Boolean; }
		case def Cs { return : Integer; }
		use case def U { return : Integer; }
		analysis an : An { return :>> v = 1; }
	}`
	returnOwnerDiags(t, constraintDiags(t, src), src)
}

// KerML: a behavior, step, classifier, structure or feature owning a `return`
// is reported; a function, expression, predicate or boolean expression is not.
func TestReturnParameterKerMLOwners(t *testing.T) {
	const invalid = `package P {
		private import ScalarValues::*;
		behavior B { return r : Integer; }
		step s { return r : Integer; }
		classifier K { return x : Integer; }
		struct St { return x : Integer; }
		feature f { return x : Integer; }
	}`
	returnOwnerDiags(t, constraintDiagsKerML(t, invalid), invalid,
		"return r : Integer;", "return r : Integer;", "return x : Integer;", "return x : Integer;", "return x : Integer;")

	const valid = `package P {
		private import ScalarValues::*;
		function F { in x : Integer; return y : Integer; }
		expr e : F { in x : Integer; return y : Integer; }
		predicate Pr { in x : Integer; return : Boolean; }
		bool b { return : Boolean; }
		function G { return : Integer = 1; }
	}`
	returnOwnerDiags(t, constraintDiagsKerML(t, valid), valid)
}
