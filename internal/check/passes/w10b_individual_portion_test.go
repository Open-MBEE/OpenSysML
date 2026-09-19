package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// The reproducer of validation/invalid/IndividualUsage_Invalid: an individual
// typed by two individual definitions, and one typed by none.
func TestW10BIndividualTypings(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		individual def A_1 :> A;
		individual def B_1 :> B;
		individual two_types : A_1, B_1;
		individual b_1_1 : B;
		individual untyped;
		individual ok : A_1;
		individual mixed : A_1, A;
	}`
	diags := only(typeDiags(t, src), "individual-typing")
	if len(diags) != 3 {
		t.Fatalf("got %d individual-typing diagnostics, want 3: %v", len(diags), diags)
	}
	want := []string{msgIndividualManyTypes, msgIndividualOneType, msgIndividualOneType}
	for i, d := range diags {
		if d.Message != want[i] {
			t.Errorf("diagnostic %d message = %q, want %q", i, d.Message, want[i])
		}
		if d.Severity != diag.SeverityError {
			t.Errorf("diagnostic %d severity = %v, want an error", i, d.Severity)
		}
	}
	// The reference reports the declaration, not the type reference.
	if got := src[diags[0].Span.Offset : diags[0].Span.Offset+len("individual")]; got != "individual" {
		t.Errorf("first diagnostic starts at %q, want the declaration", got)
	}
}

// An unresolved type is a name-resolution error, and the type tier is skipped
// after one, so this rule stays quiet; malformed input must not panic.
func TestW10BIndividualUnresolvedAndMalformed(t *testing.T) {
	if diags := only(typeDiags(t, "package P { individual x : Nope; }"), "individual-typing"); len(diags) != 0 {
		t.Fatalf("got %v, want no individual-typing diagnostic behind the name error", diags)
	}
	for _, src := range []string{"individual", "package P { individual : ; }", "individual x :"} {
		typeDiags(t, src)
	}
}

// The reproducer of validation/invalid/PortionUsage_Invalid: a portion needs an
// occurrence owner, and a package or an attribute usage is not one.
func TestW10BPortionOwner(t *testing.T) {
	const src = `package P {
		part p1 {
			snapshot s1;
			timeslice t1 { snapshot s2; }
			attribute a1 { snapshot bad; }
		}
		occurrence o1 { timeslice ok; }
		snapshot s2;
		timeslice t2;
	}`
	diags := only(constraintDiags(t, src), "portion-owner")
	if len(diags) != 3 {
		t.Fatalf("got %d portion-owner diagnostics, want 3: %v", len(diags), diags)
	}
	for _, d := range diags {
		if d.Message != msgPortionOwner {
			t.Errorf("message = %q, want %q", d.Message, msgPortionOwner)
		}
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
	}
	if !strings.HasPrefix(src[diags[0].Span.Offset:], "snapshot bad") {
		t.Errorf("first diagnostic is at %q", src[diags[0].Span.Offset:diags[0].Span.Offset+12])
	}
}
