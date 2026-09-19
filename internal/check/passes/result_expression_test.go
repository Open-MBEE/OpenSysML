package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const resultExpressionCode = "result-expression-at-most-one"

// resultExpressionDiags asserts the result-expression diagnostics of src are
// errors anchored, in order, at the wanted spans: the body a type states over
// an inherited result, or the declaration of a type inheriting two.
func resultExpressionDiags(t *testing.T, src string, wantSpans ...string) {
	t.Helper()
	diags := only(constraintDiags(t, src), resultExpressionCode)
	if len(diags) != len(wantSpans) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(wantSpans), diags)
	}
	for i, d := range diags {
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		if d.Message != msgResultExpressionAtMostOne {
			t.Errorf("message = %q, want %q", d.Message, msgResultExpressionAtMostOne)
		}
		if got := strings.TrimSpace(spanText(src, d)); got != wantSpans[i] {
			t.Errorf("span text = %q, want %q", got, wantSpans[i])
		}
	}
}

// A definition specializing one that owns a result expression may not state a
// body of its own: a constraint, a calculation or a requirement's constraint.
func TestResultExpressionSpecializedDefinitionBody(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		constraint def Base { x > 0.0 }
		constraint def Sub :> Base { x > 1.0 }
		calc def C { in y : Real; y + 1.0 }
		calc def D :> C { y + 2.0 }
		requirement def R { require constraint c { x > 0.0 } }
		requirement def R2 :> R { require constraint :>> c { x > 1.0 } }
	}`
	resultExpressionDiags(t, src, "x > 1.0", "y + 2.0", "x > 1.0")
}

// A usage typed by, subsetting or redefining a constraint or calculation that
// owns a result expression may not state a body either, wherever it nests.
func TestResultExpressionUsageBody(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		constraint def Base { x > 0.0 }
		constraint c : Base { x > 1.0 }
		constraint d :> c { x > 2.0 }
		calc def C { in y : Real; y + 1.0 }
		calc k : C { y + 2.0 }
		part def A { constraint e { x > 0.0 } }
		part def B :> A { constraint :>> e { x > 3.0 } }
		part def V :> A { assert constraint :>> e { x > 4.0 } }
	}`
	resultExpressionDiags(t, src, "x > 1.0", "x > 2.0", "y + 2.0", "x > 3.0", "x > 4.0")
}

// Reference subsetting is a specialization too: a usage referencing a constraint
// or calculation inherits its result expression and may not state a body.
func TestResultExpressionReferenceSubsettingBody(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		constraint c { x > 0.0 }
		constraint d ::> c { x > 1.0 }
		constraint kept ::> c;
		calc k { in y : Real; y + 1.0 }
		calc m ::> k { 2.0 }
	}`
	resultExpressionDiags(t, src, "x > 1.0", "2.0")
}

// A type inheriting result expressions from two generals is invalid on its own
// and is reported at its declaration; requirements and viewpoints are constraints.
func TestResultExpressionTwoInherited(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		constraint def A { x > 0.0 }
		constraint def B { x < 9.0 }
		constraint def C :> A, B;
		requirement def R :> A, B;
		concern def K :> A, B;
		viewpoint def V :> A, B;
		constraint a : A;
		constraint b : B;
		viewpoint def VD;
		viewpoint v : VD :> a, b;
		viewpoint v1 : VD :> a;
	}`
	resultExpressionDiags(t, src,
		"constraint def C :> A, B;",
		"requirement def R :> A, B;",
		"concern def K :> A, B;",
		"viewpoint def V :> A, B;",
		"viewpoint v : VD :> a, b;")
}

// A redefinition that states no result — empty, braced, documented or nesting
// an assertion — keeps the inherited one; a new constraint beside the inherited
// one and a body specializing a bodiless general are the conforming spellings.
func TestResultExpressionInheritedIsKept(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		constraint def Base { x > 0.0 }
		constraint def Sub :> Base;
		constraint def Braced :> Base { }
		constraint c : Base;
		constraint d : Base { doc /* restated */ }
		requirement def R { require constraint c { x > 0.0 } }
		requirement def Kept :> R { require constraint :>> c; }
		requirement def Tight :> R { require constraint :>> c { assert constraint { x > 1.0 } } }
		requirement def Wider :> R { require constraint d { x < 9.0 } }
		abstract calc def Shape { in y : Real; }
		calc def Round :> Shape { y * 2.0 }
		calc def Plain { in y : Real; return z : Real = y + 1.0; }
	}`
	resultExpressionDiags(t, src)
}

// The conditions one body lists are that body's single result, not several,
// and a result reached along two specialization paths counts once.
func TestResultExpressionOneBodyManyConditions(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		constraint validRange {
			in x : Real;
			x >= 0.0
			not (x > 100.0)
		}
		constraint def A { in x : Real; x > 0.0 }
		constraint def B :> A;
		constraint def C :> A;
		constraint def D :> B, C;
	}`
	resultExpressionDiags(t, src)
}

// A calculation body listing two bare expressions states two result expressions
// — the pilot's grammar admits one — and a bodiless calculation inheriting them
// is faulted at its declaration; a constraint body's conditions stay one result.
func TestResultExpressionOneBodyTwoResults(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute x : Real;
		calc c { 1.0 2.0 }
		calc def C { in y : Real; y + 1.0 y + 2.0 }
		calc def D :> C;
		calc k : C;
		constraint def K { in y : Real; y > 0.0 y < 10.0 }
		constraint m : K;
	}`
	resultExpressionDiags(t, src, "2.0", "y + 2.0", "calc def D :> C;", "calc k : C;")
}

func TestResultExpressionOneBodyTwoResultsKerML(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		feature x : Real;
		function F { in y : Real; y + 1.0 y + 2.0 }
		function G specializes F;
		expr e { 1.0 2.0 }
		expr f : F;
		predicate Q { x > 0.0 x < 10.0 }
	}`
	diags := only(constraintDiagsKerML(t, src), resultExpressionCode)
	want := []string{"y + 2.0", "function G specializes F;", "2.0", "expr f : F;"}
	if len(diags) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(want), diags)
	}
	for i, d := range diags {
		if got := strings.TrimSpace(spanText(src, d)); got != want[i] {
			t.Errorf("span text = %q, want %q", got, want[i])
		}
	}
}

// The KerML forms: a predicate, a boolean expression and an invariant hold the
// same rule; a result parameter bound with `return r = …` is a binding, not a
// second result expression.
func TestResultExpressionKerMLForms(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		feature x : Real;
		predicate Base { x > 0.0 }
		predicate Sub specializes Base { x > 1.0 }
		bool b : Base { x > 2.0 }
		inv i : Base { x > 3.0 }
		predicate Kept specializes Base;
		function F { in y : Real; y + 1.0 }
		function G specializes F { return r : Real = y + 2.0; }
		expr e { x }
		expr f ::> e { x }
		expr g ::> e;
	}`
	diags := only(constraintDiagsKerML(t, src), resultExpressionCode)
	want := []string{"x > 1.0", "x > 2.0", "x > 3.0", "x"}
	if len(diags) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(want), diags)
	}
	for i, d := range diags {
		if got := strings.TrimSpace(spanText(src, d)); got != want[i] {
			t.Errorf("span text = %q, want %q", got, want[i])
		}
	}
}
