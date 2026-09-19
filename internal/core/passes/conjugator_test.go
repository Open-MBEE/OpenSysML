package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const typeConjugatorsCode = "type-conjugators"

// A classifier, feature or type declaring two conjugations is reported once per
// conjugation beyond the first, at that conjugation, in either spelling.
func TestSecondConjugatorIsReported(t *testing.T) {
	const src = `package P {
		classifier A;
		classifier B;
		classifier C ~A ~B;
		classifier D conjugates A conjugates B;
		feature f ~A ~B;
		type T ~A ~B ~A;
	}`
	diags := only(constraintDiagsKerML(t, src), typeConjugatorsCode)
	want := []string{"~B", "conjugates B", "~B", "~B", "~A"}
	if len(diags) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(want), diags)
	}
	for i, d := range diags {
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		if d.Message != msgAtMostOneConjugator {
			t.Errorf("message = %q, want %q", d.Message, msgAtMostOneConjugator)
		}
		if got := strings.TrimSpace(spanText(src, d)); got != want[i] {
			t.Errorf("span text = %q, want %q", got, want[i])
		}
	}
}

// One conjugation is the limit, not the forbidden shape: a single conjugation,
// a standalone conjugation member and a conjugated port typing are silent.
func TestSingleConjugatorIsClean(t *testing.T) {
	const kerml = `package P {
		classifier A;
		classifier B;
		classifier C ~A;
		classifier D conjugates A;
		feature f ~A;
		conjugate C ~ B;
		conjugation cj conjugate D conjugates B;
		classifier E :> A, B;
	}`
	if diags := only(constraintDiagsKerML(t, kerml), typeConjugatorsCode); len(diags) != 0 {
		t.Errorf("got %d diagnostics, want none: %v", len(diags), diags)
	}

	const sysml = `package P {
		port def PA;
		port def PB :> PA;
		part def K { port p : ~PA; port q : ~PB; }
	}`
	if diags := only(constraintDiags(t, sysml), typeConjugatorsCode); len(diags) != 0 {
		t.Errorf("got %d diagnostics, want none: %v", len(diags), diags)
	}
}
