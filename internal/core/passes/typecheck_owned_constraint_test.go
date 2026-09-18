package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// The constraint an assume/require member owns is a constraint usage, so it is
// held to every rule an ordinary `constraint` declaration is (SysML v2 §7.20.5).

const ownedConstraintPrelude = `package L {
	part def PD;
	constraint def C;
	constraint def D;
	constraint k : C;
}
`

// ownedVsOrdinary analyzes a requirement declaring decl as an owned constraint
// (`require`, then `assume`) and as an ordinary usage, and returns the
// diagnostics of one code on each, with the ordinary form's asserted first.
func ownedVsOrdinary(t *testing.T, analyze func(*testing.T, string) []diag.Diagnostic, code, decl, wantSpan string) (ordinary, require, assume diag.Diagnostic) {
	t.Helper()
	forms := []struct{ name, keyword string }{
		{"ordinary", ""}, {"require", "require "}, {"assume", "assume "},
	}
	var out []diag.Diagnostic
	for _, form := range forms {
		src := ownedConstraintPrelude + "package P {\n\tprivate import L::*;\n\trequirement def R {\n\t\t" +
			form.keyword + "constraint " + decl + "\n\t}\n}\n"
		diags := only(analyze(t, src), code)
		if len(diags) != 1 {
			t.Fatalf("%s form: want one %s diagnostic, got %v", form.name, code, diags)
		}
		if got := strings.TrimSpace(spanText(src, diags[0])); got != wantSpan {
			t.Errorf("%s form: span text = %q, want %q", form.name, got, wantSpan)
		}
		out = append(out, diags[0])
	}
	for _, owned := range out[1:] {
		if owned.Message != out[0].Message || owned.Severity != out[0].Severity {
			t.Errorf("owned form reports %v, the ordinary form %v", owned, out[0])
		}
	}
	return out[0], out[1], out[2]
}

func TestOwnedConstraintTypedByANonConstraint(t *testing.T) {
	ordinary, _, _ := ownedVsOrdinary(t, typeDiags, "one-type", "c : PD;", "constraint c : PD;")
	if ordinary.Message != "A constraint must be typed by one constraint definition." {
		t.Errorf("message = %q", ordinary.Message)
	}
}

func TestOwnedConstraintTypedTwice(t *testing.T) {
	ordinary, _, _ := ownedVsOrdinary(t, typeDiags, "one-type", "c : C : D;", "constraint c : C : D;")
	if ordinary.Message != "A constraint must be typed by one constraint definition." {
		t.Errorf("message = %q", ordinary.Message)
	}
}

func TestOwnedConstraintIncompatibleValue(t *testing.T) {
	ordinary, _, _ := ownedVsOrdinary(t, typeDiags, "type.expr", "c : C = 5;", "5")
	if ordinary.Message != "cannot bind Natural value to a feature typed by C" {
		t.Errorf("message = %q", ordinary.Message)
	}
}

func TestOwnedConstraintInvertedMultiplicity(t *testing.T) {
	ordinary, _, _ := ownedVsOrdinary(t, constraintDiags, "multiplicity-range", "c : C [3..1];", "[3..1]")
	if !strings.Contains(ordinary.Message, "lower bound exceeds upper bound on c") {
		t.Errorf("message = %q", ordinary.Message)
	}
}

// A redefining owned constraint inherits the multiplicity of the constraint it
// redefines, whichever form declared that one.
func TestOwnedConstraintValueCountThroughRedefinition(t *testing.T) {
	const src = ownedConstraintPrelude + `package P {
		private import L::*;
		requirement def R {
			require constraint c1 : C [2];
			constraint c2 : C [2];
		}
		requirement def S :> R {
			require constraint :>> c1 = (k, k);
			constraint :>> c2 = (k, k);
		}
		requirement def T :> R {
			require constraint :>> c1 = (true, true, true);
			constraint :>> c2 = (true, true, true);
		}
	}`
	var counts []string
	for _, d := range typeDiags(t, src) {
		if strings.Contains(d.Message, "multiplicity upper bound") {
			counts = append(counts, spanText(src, d))
		}
	}
	// Only literals have a statically known count: a feature reference may be
	// multi-valued.
	if len(counts) != 2 || counts[0] != "(true, true, true)" || counts[1] != "(true, true, true)" {
		t.Fatalf("want the two over-long collections reported, got %v", counts)
	}
}

// The reference and condition forms declare no constraint usage of their own,
// so they are not held to the declaration rules.
func TestOwnedConstraintReferenceFormIsNotADeclaration(t *testing.T) {
	const src = ownedConstraintPrelude + `package P {
		private import L::*;
		requirement def Q;
		requirement def R {
			require k : C : D [3..1];
			assume Q;
		}
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Errorf("reference form reported as a declaration: %v", diags)
	}
	if diags := only(constraintDiags(t, src), "multiplicity-range"); len(diags) != 0 {
		t.Errorf("reference form reported as a declaration: %v", diags)
	}
}

// A requirement, concern or viewpoint definition is a constraint definition
// (SysML v2 §8.3.19), so it types a constraint usage as the pilot accepts.
func TestConstraintTypedByARequirementDefinition(t *testing.T) {
	src := `package P {
	attribute def Real;
	requirement def R { subject s : Real; in x : Real; require constraint { x > 0 } }
	concern def Cn { subject s : Real; }
	viewpoint def Vp { subject s : Real; }
	constraint c1 : R;
	constraint c2 : Cn;
	constraint c3 : Vp;
	requirement def Q {
		subject s : Real;
		require constraint c4 : R { in x = 1; }
		assert constraint c5 : R { in x = 1; }
	}
	part def PD;
	constraint c6 : PD;
}
`
	diags := only(typeDiags(t, src), "one-type")
	if len(diags) != 1 {
		t.Fatalf("want one one-type diagnostic, for c6 alone, got %v", diags)
	}
	if got := strings.TrimSpace(spanText(src, diags[0])); got != "constraint c6 : PD;" {
		t.Errorf("span text = %q, want the constraint typed by a part definition", got)
	}
}
