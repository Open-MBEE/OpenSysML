package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestMultiplicityDomainStandaloneFeaturingIsAnError(t *testing.T) {
	const src = `package P {
		class C {
			feature x {
				multiplicity m [1];
			}
		}
		class D;
		featuring of C::x::m by D;
	}`
	diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type")
	if len(diags) != 1 {
		t.Fatalf("expected one multiplicity-domain diagnostic, got %v", diags)
	}
	if diags[0].Message != msgMultiplicityFeaturingTypes {
		t.Fatalf("got message %q, want %q", diags[0].Message, msgMultiplicityFeaturingTypes)
	}
	if diags[0].Severity != diag.SeverityError {
		t.Fatalf("got severity %v, want error", diags[0].Severity)
	}
}

func TestMultiplicityDomainBodyFeaturingIsAnError(t *testing.T) {
	const src = `package P {
		class D;
		class C {
			feature x {
				multiplicity m [1];
			}
			featuring of x::m by D;
		}
	}`
	if diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type"); len(diags) != 1 {
		t.Fatalf("expected one multiplicity-domain diagnostic, got %v", diags)
	}
}

func TestMultiplicityDomainFeaturingItsOwnerIsSilent(t *testing.T) {
	const src = `package P {
		class C {
			feature x {
				multiplicity m [1];
			}
		}
		featuring of C::x::m by C;
	}`
	if diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type"); len(diags) != 0 {
		t.Fatalf("featuring a multiplicity by its feature owner is valid, got %v", diags)
	}
}

func TestMultiplicityDomainFeaturingTheFeatureIsNotThisRule(t *testing.T) {
	const src = `package P {
		class C { feature x; }
		class D;
		featuring of C::x by D;
	}`
	if diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type"); len(diags) != 0 {
		t.Fatalf("featuring the feature itself is not a multiplicity-domain check, got %v", diags)
	}
}

func TestMultiplicityDomainClassifierMultiplicityIsNotThisRule(t *testing.T) {
	const src = `package P {
		class C { multiplicity m [1]; }
		class D;
		featuring of C::m by D;
	}`
	if diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type"); len(diags) != 0 {
		t.Fatalf("classifier-owned multiplicities are checked by another rule, got %v", diags)
	}
}

func TestMultiplicityDomainUnresolvedTargetIsSilent(t *testing.T) {
	const src = `package P {
		class C {
			feature x {
				multiplicity m [1];
			}
		}
		featuring of C::x::m by Missing;
	}`
	if diags := only(constraintDiagsKerML(t, src), "multiplicity-featuring-type"); len(diags) != 0 {
		t.Fatalf("an unresolved featuring target is handled by name resolution, got %v", diags)
	}
}
