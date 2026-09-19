package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// endFeatureLines returns the 1-based source lines on which src draws the
// end-feature diagnostic with the given code, in order.
func endFeatureLines(t *testing.T, src, code, message string, kerml bool) []int {
	t.Helper()
	var diags []diag.Diagnostic
	if kerml {
		diags = constraintDiagsKerML(t, src)
	} else {
		diags = constraintDiags(t, src)
	}
	var lines []int
	for _, d := range only(diags, code) {
		if d.Message != message {
			t.Errorf("message = %q, want %q", d.Message, message)
		}
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

// A SysML end usage may spell a direction or the derived, abstract and
// variation prefixes; the pilot rejects each (validateFeatureEndNoDirection,
// validateFeatureEndNotDerivedAbstractCompositeOrPortion) while `end part`
// and `end ref` ends pass, an end usage being referential.
func TestEndFeatureSysML(t *testing.T) {
	const src = `package P {
		part def A;
		connection def K1 { end in a : A; end b : A; }
		connection def K2 { end out a : A; end b : A; }
		connection def K3 { end inout a : A; end b : A; }
		connection def K4 { end derived a : A; end b : A; }
		connection def K5 { end abstract a : A; end b : A; }
		connection def K6 { end part a : A; end b : A; }
		connection def K7 { end derived abstract a : A; end b : A; }
		connection def K8 { end in derived a : A; end b : A; }
		connection def K9 { end a : A; end b : A; }
		connection def K10 { end ref a : A; end b : A; }
		connection def K11 { end variation a : A; end b : A; }
		part def H { in a : A; derived b : A; abstract c : A; }
	}`
	if got, want := endFeatureLines(t, src, "end-feature-direction", msgEndFeatureDirection, false), []int{3, 4, 5, 10}; !sameLines(got, want) {
		t.Fatalf("end-feature-direction lines = %v, want %v", got, want)
	}
	if got, want := endFeatureLines(t, src, "end-feature-kind", msgEndFeatureKind, false), []int{6, 7, 9, 10, 13}; !sameLines(got, want) {
		t.Fatalf("end-feature-kind lines = %v, want %v", got, want)
	}
}

// KerML's end-feature prefix admits only `const`, so a KerML end never carries
// a direction or one of the rejected prefixes; ordinary features are not ends.
func TestEndFeatureKerMLSilent(t *testing.T) {
	const src = `package P {
		class A;
		assoc K { end feature a : A; end feature b : A; }
		class C { in feature x : A; derived feature y : A; abstract feature z : A; composite feature w : A; portion feature v : A; }
	}`
	if got := endFeatureLines(t, src, "end-feature-direction", msgEndFeatureDirection, true); len(got) != 0 {
		t.Fatalf("end-feature-direction lines = %v, want none", got)
	}
	if got := endFeatureLines(t, src, "end-feature-kind", msgEndFeatureKind, true); len(got) != 0 {
		t.Fatalf("end-feature-kind lines = %v, want none", got)
	}
}

// The prefix between `end` and a cross feature's declaration is the cross
// feature's: the pilot accepts `end in x1 : A [1] item x : A`.
func TestEndFeaturePrefixBelongsToCrossFeature(t *testing.T) {
	const sysml = `package P {
		part def A;
		connection def K1 { end in x1 : A [1] item x : A; end b : A; }
		connection def K2 { end derived x1 [1] item x : A; end b : A; }
		connection def K3 { end variation x1 : A [1] item x : A; end b : A; }
		connection def K4 { end in derived ref x1 : A [1] item x : A; end b : A; }
		connection def K5 { end in a : A; end derived b : A; }
	}`
	if got, want := endFeatureLines(t, sysml, "end-feature-direction", msgEndFeatureDirection, false), []int{7}; !sameLines(got, want) {
		t.Fatalf("end-feature-direction lines = %v, want %v", got, want)
	}
	if got, want := endFeatureLines(t, sysml, "end-feature-kind", msgEndFeatureKind, false), []int{7}; !sameLines(got, want) {
		t.Fatalf("end-feature-kind lines = %v, want %v", got, want)
	}

	const kerml = `package P {
		class A;
		assoc K1 { end in x1 : A [1] feature x : A; end feature b : A; }
		assoc K2 { end derived abstract composite x2 : A [1] feature x : A; end feature b : A; }
		assoc K3 { end out [1] feature x : A; end feature b : A; }
	}`
	if got := endFeatureLines(t, kerml, "end-feature-direction", msgEndFeatureDirection, true); len(got) != 0 {
		t.Fatalf("end-feature-direction lines = %v, want none", got)
	}
	if got := endFeatureLines(t, kerml, "end-feature-kind", msgEndFeatureKind, true); len(got) != 0 {
		t.Fatalf("end-feature-kind lines = %v, want none", got)
	}
}
