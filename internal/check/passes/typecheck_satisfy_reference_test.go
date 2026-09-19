package passes

import "testing"

// The reference form of a satisfy requirement usage must reference a requirement
// usage (SysML v2 §8.3.19, validateSatisfyRequirementUsageReference). The pilot
// judges the last feature of a chain; a concern, viewpoint, objective or satisfy
// usage is a requirement usage.
func TestSatisfyReferenceToRequirementIsSilent(t *testing.T) {
	src := `requirement def RD; concern def CD; viewpoint def VD;
part def Base { requirement inherited : RD; requirement kept : RD; concern bc : CD; }
part def Derived :> Base { requirement r : RD; requirement r2 :>> inherited; requirement nested : RD { requirement sub : RD; } }
part v : Derived; requirement top : RD; alias atop for top; viewpoint vp : VD;
verification def VF { objective obj : RD; }
verification def VF2 :> VF { satisfy obj; satisfy VF2::obj; }
part def Owner :> Base {
	satisfy inherited;
	satisfy bc;
	satisfy top;
	satisfy atop;
	satisfy vp;
	satisfy v.r;
	satisfy v.r2;
	satisfy v.kept;
	satisfy v.bc;
	satisfy v.nested.sub;
	satisfy requirement s : RD;
	satisfy s;
	satisfy top by v;
	satisfy top by v.r;
	assert satisfy top;
	assert not satisfy top;
}
requirement def RQ { satisfy top; }
use case def UC { satisfy top; }
constraint def K { satisfy top; true }
action def A { satisfy top; }
state def S { satisfy top; }
calc def C { satisfy top; 1 }`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

func TestSatisfyReferenceToNonRequirementRejected(t *testing.T) {
	tests := []struct {
		name, target, found string
	}{
		{"constraint usage", "satisfy c;", "constraintUsage"},
		{"asserted negated constraint usage", "assert not satisfy c;", "constraintUsage"},
		{"inherited constraint usage", "satisfy bc;", "constraintUsage"},
		{"chained constraint usage", "satisfy v.c;", "constraintUsage"},
		{"chained inherited part", "satisfy v.bp;", "partUsage"},
		{"chain ending in a requirement's part", "satisfy top.q;", "partUsage"},
		{"redefinition by a part", "satisfy v.rp;", "partUsage"},
		{"reference typed by a requirement def", "satisfy rr;", "attributeUsage"},
		{"reference subsetting a requirement", "satisfy rs;", "attributeUsage"},
		{"attribute usage", "satisfy a;", "attributeUsage"},
		{"item usage", "satisfy i;", "itemUsage"},
		{"action usage", "satisfy act;", "actionUsage"},
		{"alias to constraint", "satisfy ac;", "constraintUsage"},
		{"requirement definition", "satisfy RD;", "requirementDef"},
		{"in a requirement body", "} requirement def RQ { satisfy c;", "constraintUsage"},
		{"in a use case body", "} use case def UC { satisfy c;", "constraintUsage"},
		{"in an objective body", "} verification def VF { objective o : RD { satisfy c; }", "constraintUsage"},
		{"in a constraint body", "} constraint def K { satisfy c; true", "constraintUsage"},
		{"in a state body", "} state def S { satisfy c;", "constraintUsage"},
		{"in an action body", "} action def A { satisfy c;", "constraintUsage"},
		{"in a calc body", "} calc def C { satisfy c; 1", "constraintUsage"},
	}
	prefix := `constraint def CD; requirement def RD; item def ID; part def PD;
part def Base { part bp; constraint bc : CD; requirement br : RD; }
part def Derived :> Base { constraint c : CD; part rp :>> br; }
part v : Derived; constraint c : CD; alias ac for c; attribute a; item i : ID; action act;
requirement top : RD { part q : PD; }
ref rr : RD; ref rs :> top;
part def Owner :> Base { `
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			want := "satisfy target must be a requirement usage, found " + tc.found
			if diags[0].Message != want {
				t.Errorf("got %q, want %q", diags[0].Message, want)
			}
		})
	}
}
