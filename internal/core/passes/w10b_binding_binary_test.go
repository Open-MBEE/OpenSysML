package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// bindingBinaryLines returns the 1-based source lines on which src draws a
// binding-binary diagnostic, in order.
func bindingBinaryLines(t *testing.T, src string, kerml bool) []int {
	t.Helper()
	var diags []diag.Diagnostic
	if kerml {
		diags = constraintDiagsKerML(t, src)
	} else {
		diags = constraintDiags(t, src)
	}
	var lines []int
	for _, d := range only(diags, "binding-binary") {
		if d.Message != msgBindingBinary {
			t.Errorf("message = %q, want %q", d.Message, msgBindingBinary)
		}
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

// A KerML binding relates exactly two features: the pilot reports one with a
// body end past its clause pair, with one end only, or with none, abstract or
// not (validateBindingConnectorIsBinary), and is silent on the two-ended forms.
func TestW10BBindingBinaryKerML(t *testing.T) {
	const src = `package P {
		class C {
			feature x; feature y; feature z;
			binding x = y;
			binding b2 of x = y;
			binding b3 of x = y { end feature e3 references z; }
			binding b4 { end feature e1 references x; end feature e2 references y; }
			binding b5 { end feature e1 references x; }
			binding b6;
			abstract binding b7;
		}
		class D :> C {
			binding b8 :>> b6 of x = y;
		}
	}`
	if got, want := bindingBinaryLines(t, src, true), []int{6, 8, 9, 10}; !sameLines(got, want) {
		t.Fatalf("binding-binary lines = %v, want %v", got, want)
	}
}

// The SysML `bind` clause states both ends, one of them as the value; a body end
// beside it makes three.
func TestW10BBindingBinarySysML(t *testing.T) {
	const src = `package P {
		part def C {
			attribute x; attribute y; attribute z;
			bind x = y;
			binding b2 bind x = y;
			binding b3 bind x = y { end e3 references z; }
			binding b6;
		}
		part def D :> C {
			binding b8 :>> b6 bind x = y;
		}
	}`
	if got, want := bindingBinaryLines(t, src, false), []int{6, 7}; !sameLines(got, want) {
		t.Fatalf("binding-binary lines = %v, want %v", got, want)
	}
}

// An end naming a classifier is still a stated end: the pilot reports it as an
// unresolved feature and counts it, so a two-ended binding to a definition is
// not reported as non-binary and a three-ended one is.
func TestW10BBindingBinaryCountsNonFeatureEnds(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		part x : A;
		binding b1 bind x = B;
		part def C { binding b3 bind x = B { end e references x; } }
	}`
	diags := constraintDiags(t, src)
	if got, want := bindingBinaryLines(t, src, false), []int{6}; !sameLines(got, want) {
		t.Fatalf("binding-binary lines = %v, want %v", got, want)
	}
	var referent []int
	for _, d := range only(diags, "feature-reference-referent") {
		referent = append(referent, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	if want := []int{5, 6}; !sameLines(referent, want) {
		t.Fatalf("feature-reference-referent lines = %v, want %v", referent, want)
	}
}

// A binding that inherits its two ends from the binding it subsets is binary.
func TestW10BBindingInheritingTwoEndsIsBinary(t *testing.T) {
	const src = `package P {
		class C {
			feature x; feature y;
			binding b1 of x = y;
			binding b2 :> b1;
		}
	}`
	if got := bindingBinaryLines(t, src, true); len(got) != 0 {
		t.Fatalf("binding inheriting two ends reported on lines %v", got)
	}
}
