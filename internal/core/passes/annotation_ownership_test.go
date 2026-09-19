package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// annotationOwnershipLines returns the 1-based source lines on which src draws
// an annotation-ownership diagnostic, in order.
func annotationOwnershipLines(t *testing.T, src string, kerml bool) []int {
	t.Helper()
	var diags []diag.Diagnostic
	if kerml {
		diags = constraintDiagsKerML(t, src)
	} else {
		diags = constraintDiags(t, src)
	}
	var lines []int
	for _, d := range only(diags, "annotation-annotated-element-ownership") {
		if d.Message != msgAnnotationOwnsAnnotating {
			t.Errorf("message = %q, want %q", d.Message, msgAnnotationOwnsAnnotating)
		}
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

// An annotating element that names itself in its `about` clause owns an
// annotation whose annotated element is the annotating element, which then
// fails to own it (validateAnnotationAnnotatedElementOwnership).
func TestAnnotationOwnershipKerML(t *testing.T) {
	const src = `package P {
		class A;
		metaclass M;
		comment C1 about C1 /* self */
		metadata m2 : M about m2;
		comment C2 about A, C2 /* mixed */
		class C { comment C3 about C3 /* nested */ }
		@m4 : M about m4, A;
		alias C1Alias for C1;
		comment C5 about C1Alias /* other comment */
		comment C6 about C6Alias /* self through alias */
		alias C6Alias for C6;
		comment about A /* owned by the package */
		metadata m7 : M about A;
		class D { comment about D /* nested, about the owner */ }
	}`
	if got, want := annotationOwnershipLines(t, src, true), []int{4, 5, 6, 7, 8, 11}; !sameLines(got, want) {
		t.Fatalf("annotation-ownership lines = %v, want %v", got, want)
	}
}

func TestAnnotationOwnershipSysML(t *testing.T) {
	const src = `package P {
		part def A;
		metadata def M;
		comment C1 about C1 /* self */
		comment C2 about A, C2 /* mixed */
		part def C { comment C3 about C3 /* nested */ }
		metadata m4 : M about m4;
		comment about A /* owned by the package */
		metadata m5 : M about A;
	}`
	if got, want := annotationOwnershipLines(t, src, false), []int{4, 5, 6, 7}; !sameLines(got, want) {
		t.Fatalf("annotation-ownership lines = %v, want %v", got, want)
	}
}

// An unresolved reference elsewhere in the document gates only the annotation
// that states it; a self-annotation beside it is still reported.
func TestAnnotationOwnershipSurvivesUnrelatedFailures(t *testing.T) {
	const sysml = `package P {
		part def A;
		metadata def M;
		part unrelated : Missing;
		comment C1 about C1 /* self */
		comment C2 about Missing /* unresolved, no cascade */
		metadata m3 : M about Missing, m3;
		part def C { comment about Missing /* owned by C */ }
	}`
	if got, want := annotationOwnershipLines(t, sysml, false), []int{5, 7}; !sameLines(got, want) {
		t.Fatalf("SysML annotation-ownership lines = %v, want %v", got, want)
	}
	const kerml = `package P {
		class A;
		metaclass M;
		feature unrelated : Missing;
		comment C1 about C1 /* self */
		comment C2 about Missing /* unresolved, no cascade */
		@m3 : M about Missing, m3;
	}`
	if got, want := annotationOwnershipLines(t, kerml, true), []int{5, 7}; !sameLines(got, want) {
		t.Fatalf("KerML annotation-ownership lines = %v, want %v", got, want)
	}
}
