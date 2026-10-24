package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestW7GSubsettingAFeatureOfAnotherTypeIsAnError(t *testing.T) {
	const src = `package E {
		part def P { attribute n; }
		part def R { attribute m :> E::P::n; }
		attribute top :> E::P::n;
	}`
	diags := only(constraintDiags(t, src), "subsetting-featuring-types")
	if len(diags) != 2 {
		t.Fatalf("expected both inaccessible subsettings to be reported, got %v", diags)
	}
	for _, d := range diags {
		if d.Message != msgSubsettingFeaturingTypes {
			t.Fatalf("expected the reference's wording, got %q", d.Message)
		}
		if d.Severity != diag.SeverityError {
			t.Fatalf("the reference reports an error, got %v", d.Severity)
		}
	}
}

func TestW7GSubsettingThroughTheFeaturingContextIsSilent(t *testing.T) {
	const src = `package E {
		part def P { attribute n; }
		part p : P { attribute k :> n; }
		part def S { part inner : P { attribute j :> n; } }
	}`
	if diags := only(constraintDiags(t, src), "subsetting-featuring-types"); len(diags) != 0 {
		t.Fatalf("a feature reachable through the featuring context is accessible, got %v", diags)
	}
}

func TestW7GSubsettingAPackageLevelFeatureIsSilent(t *testing.T) {
	const src = `package E {
		attribute base;
		part def R { attribute m :> base; }
	}`
	if diags := only(constraintDiags(t, src), "subsetting-featuring-types"); len(diags) != 0 {
		t.Fatalf("a package-level feature has no featuring type, got %v", diags)
	}
}

func TestW7GSubclassificationIsNotASubsetting(t *testing.T) {
	const src = `package E {
		part def P { part def Inner; }
		part def Q :> E::P::Inner;
	}`
	if diags := only(constraintDiags(t, src), "subsetting-featuring-types"); len(diags) != 0 {
		t.Fatalf("`:>` between classifiers is a subclassification, got %v", diags)
	}
}

func TestW7GVerificationReferenceChainIsAccessible(t *testing.T) {
	const src = `package E {
		requirement def R { requirement nested; }
		verification def V {
			objective {
				verify r.nested;
			}
		}
		requirement r : R;
	}`
	if diags := only(constraintDiags(t, src), "subsetting-featuring-types"); len(diags) != 0 {
		t.Fatalf("a dotted verification feature chain is accessible, got %v", diags)
	}
}

func TestW7GQualifiedSatisfyTargetMustBeAccessible(t *testing.T) {
	const src = `package E {
		requirement def R { requirement nested; }
		requirement r : R;
		part p;
		satisfy R::nested by p;
	}`
	diags := only(constraintDiags(t, src), "subsetting-featuring-types")
	if len(diags) != 1 || diags[0].Message != msgSubsettingFeaturingTypes {
		t.Fatalf("a qualified nested satisfy target is inaccessible, got %v", diags)
	}
}

// `specialization subset f :> g;` is the Subsetting written as a member of its
// own, so an inaccessible subsetted feature is reported there as on `:>`.
func TestW7GStandaloneSubsettingOfInaccessibleFeatureIsAnError(t *testing.T) {
	const src = `library package E {
		private import ScalarValues::*;
		struct A { feature a : Real; }
		feature fu : A;
		specialization subset fu :> A::a;
		feature fv : Real :> A::a;
	}`
	diags := only(constraintDiagsKerML(t, src), "subsetting-featuring-types")
	if len(diags) != 2 {
		t.Fatalf("expected the standalone and the inline subsetting to be reported, got %v", diags)
	}
	for i, want := range []string{"A::a", "A::a"} {
		if got := spanText(src, diags[i]); got != want {
			t.Errorf("diagnostic %d at %q, want the subsetted name", i, got)
		}
	}
}

func TestW7GStandaloneSubsettingThroughTheFeaturingContextIsSilent(t *testing.T) {
	const src = `package E {
		struct P { feature n; }
		feature p : P { feature k; specialization subset k :> n; }
		feature base;
		feature q;
		specialization subset q :> base;
	}`
	if diags := only(constraintDiagsKerML(t, src), "subsetting-featuring-types"); len(diags) != 0 {
		t.Fatalf("a reachable or package-level subsetted feature is accessible, got %v", diags)
	}
}
