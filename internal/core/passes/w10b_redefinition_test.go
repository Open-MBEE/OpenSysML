package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// spanText returns the source text a diagnostic covers, so a test can assert
// the location the reference names rather than an offset.
func spanText(src string, d diag.Diagnostic) string {
	if d.Span.Offset < 0 || d.Span.End() > len(src) {
		return ""
	}
	return src[d.Span.Offset:d.Span.End()]
}

// Two package-level features cannot redefine one another: both are featured by
// Anything (validation/invalid/Redefinition_OwningType_Invalid).
func TestW10BPackageLevelRedefinitionIsReported(t *testing.T) {
	const src = `package P {
		part def Wheel;
		part wheel : Wheel;
		part wheel1 redefines wheel;
	}`
	diags := only(constraintDiags(t, src), "redefinition-package-level")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRedefinePackageLevel {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRedefinePackageLevel)
	}
	if diags[0].Severity != diag.SeverityError {
		t.Errorf("severity = %v, want an error", diags[0].Severity)
	}
	// The diagnostic sits on the redefined reference, as in the reference.
	if got := spanText(src, diags[0]); got != "wheel" {
		t.Errorf("span text = %q, want %q", got, "wheel")
	}
}

// A package-level feature redefined from inside a type is well-formed: the
// featuring types differ (validation/valid/Redefinition_OwningType).
func TestW10BPackageLevelRedefinitionFromTypeIsClean(t *testing.T) {
	const src = `package P {
		part def Wheel;
		part wheel : Wheel;
		part def Vehicle {
			part redefines wheel;
		}
	}`
	if diags := only(constraintDiags(t, src), "redefinition-package-level"); len(diags) != 0 {
		t.Fatalf("redefining from inside a type is legal, got %v", diags)
	}
}

// A redefinition may not share its featuring type with the redefined feature,
// which an alias hop does not hide.
func TestW10BSameFeaturingTypeIsReported(t *testing.T) {
	const src = `package P {
		part def Engine;
		part def B {
			alias eng1 for Vehicle::eng;
		}
		part def Vehicle specializes B {
			part eng : Engine;
			part smallEng : Engine redefines eng1;
		}
	}`
	diags := only(constraintDiags(t, src), "redefinition-same-featuring")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRedefineSameFeaturing {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRedefineSameFeaturing)
	}
	if got := spanText(src, diags[0]); got != "eng1" {
		t.Errorf("span text = %q, want %q", got, "eng1")
	}
}

// Redefining a feature inherited from a supertype is well-formed.
func TestW10BInheritedRedefinitionIsClean(t *testing.T) {
	const src = `package P {
		part def Engine;
		part def Vehicle { part eng : Engine; }
		part def SmallVehicle :> Vehicle { part smallEng : Engine redefines eng; }
	}`
	if diags := only(constraintDiags(t, src), "redefinition-same-featuring"); len(diags) != 0 {
		t.Fatalf("inherited redefinition is legal, got %v", diags)
	}
}

// An end feature can only be redefined by an end feature.
func TestW10BEndFeatureRedefinitionIsReported(t *testing.T) {
	const src = `package RedefinitionEnd {
		part def A { end part e; }
		part def B :> A { part :>> e; }
	}`
	diags := only(constraintDiags(t, src), "redefinition-not-end")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRedefineEndFeature {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRedefineEndFeature)
	}
	if diags[0].Severity != diag.SeverityError {
		t.Errorf("severity = %v, want an error", diags[0].Severity)
	}
}

// An end feature redefined by an end feature is well-formed.
func TestW10BEndRedefiningEndIsClean(t *testing.T) {
	const src = `package RedefinitionEnd {
		part def A { end part e; }
		part def B :> A { end part :>> e; }
	}`
	if diags := only(constraintDiags(t, src), "redefinition-not-end"); len(diags) != 0 {
		t.Fatalf("an end redefining an end is legal, got %v", diags)
	}
}

// Malformed input must not panic and must not invent a redefinition target.
func TestW10BRedefinitionMalformedInput(t *testing.T) {
	const src = `package P {
		part wheel1 redefines ;
		part def { part :>> ; }
	}`
	for _, code := range []string{"redefinition-package-level", "redefinition-same-featuring", "redefinition-not-end"} {
		if diags := only(constraintDiags(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on malformed input: %v", code, diags)
		}
	}
}
