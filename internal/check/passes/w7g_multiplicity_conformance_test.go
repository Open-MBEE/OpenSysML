package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// codesOf counts the diagnostics of one code and returns their severities.
func multiplicityDiags(t *testing.T, src, code string) []diag.Diagnostic {
	t.Helper()
	return only(constraintDiags(t, src), code)
}

func TestW7GSubsettingUpperBoundIsAWarning(t *testing.T) {
	const src = `package M {
		part def P {
			part cap[0..5];
			part few[0..9] subsets cap;
		}
	}`
	diags := multiplicityDiags(t, src, "subsetting-multiplicity")
	if len(diags) != 1 {
		t.Fatalf("expected one upper-bound diagnostic, got %v", diags)
	}
	if diags[0].Severity != diag.SeverityWarning {
		t.Fatalf("expected a warning, got %v", diags[0].Severity)
	}
	if diags[0].Message != msgSubsettingMultiplicityConformance {
		t.Fatalf("message is not the reference's: %q", diags[0].Message)
	}
}

func TestW7GRedefinitionLowerAndUpperBoundsAreSeparateWarnings(t *testing.T) {
	const src = `package M {
		part def P { part cyl[2..4]; }
		part def Q :> P { part mycyl[1..8] redefines cyl; }
	}`
	diags := constraintDiags(t, src)
	if got := len(only(diags, "redefinition-multiplicity")); got != 1 {
		t.Fatalf("expected one lower-bound warning, got %d in %v", got, diags)
	}
	if got := len(only(diags, "subsetting-multiplicity")); got != 1 {
		t.Fatalf("expected one upper-bound warning, got %d in %v", got, diags)
	}
	for _, d := range diags {
		if d.Severity != diag.SeverityWarning {
			t.Fatalf("multiplicity conformance is a warning in the reference, got %v", d)
		}
	}
}

func TestW7GRedefinitionWithinTheBoundsIsSilent(t *testing.T) {
	const src = `package M {
		part def P { part cyl[2..4]; }
		part def Q :> P { part mycyl[3..4] redefines cyl; }
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("a conforming redefinition should be silent, got %v", diags)
	}
}

func TestW7GDefaultMultiplicityAppliesToPartAttributeItemAndPort(t *testing.T) {
	const src = `package M {
		port def Pt;
		item def It;
		part def P {
			attribute a;
			item i : It;
			part p;
			port t : Pt;
		}
		part def Q :> P {
			attribute a2[2] subsets a;
			item i2[2] : It subsets i;
			part p2[2] subsets p;
			port t2[2] : Pt subsets t;
		}
	}`
	if got := len(multiplicityDiags(t, src, "subsetting-multiplicity")); got != 4 {
		t.Fatalf("expected the implicit 1..1 of all four kinds to be exceeded, got %d", got)
	}
}

func TestW7GNoDefaultMultiplicityForActionsStatesOrKerMLFeatures(t *testing.T) {
	const src = `package M {
		action def A;
		state def S;
		part def P {
			action a : A;
			state s : S;
			feature f;
		}
		part def Q :> P {
			action a2[2] : A subsets a;
			state s2[2] : S subsets s;
			feature f2[2] subsets f;
		}
	}`
	if diags := multiplicityDiags(t, src, "subsetting-multiplicity"); len(diags) != 0 {
		t.Fatalf("the reference gives these kinds no default multiplicity, got %v", diags)
	}
}

func TestW7GEndAndNonEndFeaturesAreNotCompared(t *testing.T) {
	const src = `package M {
		part def P {
			part parts[0..4];
			connection c {
				end p1[0..*] subsets parts;
				end p2[0..*] subsets parts;
			}
		}
	}`
	if diags := multiplicityDiags(t, src, "subsetting-multiplicity"); len(diags) != 0 {
		t.Fatalf("an end subsetting a non-end is exempt in the reference, got %v", diags)
	}
}

func TestW7GUnboundedSubsettedUpperBoundIsSilent(t *testing.T) {
	const src = `package M {
		part def P {
			part all[0..*];
			part some[0..7] subsets all;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("an unbounded upper bound admits any upper bound, got %v", diags)
	}
}

func TestW7GSubsettingLowerBoundIsNotDiagnosed(t *testing.T) {
	const src = `package M {
		part def P {
			part cap[2..4];
			part few[0..4] subsets cap;
		}
	}`
	if diags := multiplicityDiags(t, src, "redefinition-multiplicity"); len(diags) != 0 {
		t.Fatalf("the lower-bound rule is a redefinition rule only, got %v", diags)
	}
}
