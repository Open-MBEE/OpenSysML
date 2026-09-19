package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// A concrete connector with one end relates one element, so it is reported
// where the pilot reports it (validation/invalid/Relationship_invalid_relatedElement0).
func TestW10BConnectorWithOneEndIsReported(t *testing.T) {
	const src = `package P {
		part v {
			part b0;
			connection { end ::> b0; }
		}
	}`
	diags := only(constraintDiags(t, src), "related-elements")
	if len(diags) != 1 {
		t.Fatalf("got %d related-elements diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRelatedElements {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRelatedElements)
	}
	if diags[0].Severity != diag.SeverityError {
		t.Errorf("severity = %v, want an error", diags[0].Severity)
	}
}

// An abstract connector may leave its ends to a specialization, as in the
// reference, and a connection definition with no end at all relates nothing.
func TestW10BAbstractIsExemptAndEmptyDefinitionIsReported(t *testing.T) {
	const src = `package P {
		part v {
			part b0;
			abstract connection { end ::> b0; }
		}
		connection def FuelLine;
	}`
	diags := only(constraintDiags(t, src), "related-elements")
	if len(diags) != 1 {
		t.Fatalf("got %d related-elements diagnostics, want 1 (the definition): %v", len(diags), diags)
	}
}

// Two ends relate two elements, whether declared in a `connect` clause or as
// end features of the body.
func TestW10BTwoEndsAreClean(t *testing.T) {
	const src = `package P {
		part def A;
		part a1 : A;
		part a2 : A;
		connection c connect a1 to a2;
		connection def D { end e1 : A; end e2 : A; }
	}`
	if diags := only(constraintDiags(t, src), "related-elements"); len(diags) != 0 {
		t.Fatalf("two-ended connectors are well-formed, got %v", diags)
	}
}

func TestW10BAllocationDefinitionsInheritTwoEnds(t *testing.T) {
	const src = `package P {
		part def X;
		allocation def Empty;
		allocation def One {
			end item : X;
		}
	}`
	for _, d := range constraintDiags(t, src) {
		if d.Message == msgRelatedElements {
			t.Fatalf("allocation definitions inherit two related ends, got %v", d)
		}
	}
}
