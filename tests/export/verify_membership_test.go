package export_test

import (
	"strings"
	"testing"
)

const verifyModel = `package V {
    requirement r;
    requirement def Q;
    verification def T { objective { verify r; } }
    verification def U { objective o { verify requirement v : Q; } }
    requirement def P { requirement req : Q; }
    verification def W { objective : P { verify r :>> req; } }
    verification def X { objective { verify requirement :> r; } }
    part def S;
    part s : S;
    requirement q : Q;
    satisfy q by s;
}
`

// An objective's `verify r;`, `verify r :>> req;`, `verify requirement :> r;` or
// `verify requirement v : Q;` is the
// RequirementUsage a RequirementVerificationMembership owns (SysML-textual-bnf
// RequirementVerificationMember), not a SatisfyRequirementUsage, and comes back
// as `verify` from the graph alone; a `satisfy` stays a SatisfyRequirementUsage.
func TestVerifyIsARequirementVerificationMembership(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "v.sysml", []byte(verifyModel))
	graph := string(turtle)
	for _, want := range []string{
		"elmt:V__T___400___400\n    a sysml:RequirementUsage ;",
		"elmt:V__T___400___400_om\n    a sysml:RequirementVerificationMembership ;",
		"elmt:V__U__o__v\n    a sysml:RequirementUsage ;",
		"elmt:V__U__o__v_om\n    a sysml:RequirementVerificationMembership ;",
		// A reference's subsetting is a ReferenceSubsetting, a declaration's `:>` a Subsetting.
		"elmt:V__W___400___400_rs\n    a sysml:ReferenceSubsetting ;",
		"elmt:V__X___400___400_ss0\n    a sysml:Subsetting ;",
		"sysml:ownedRequirement elmt:V__T___400___400 ;",
		`sysml:kind "requirement" ;`,
		"a sysml:SatisfyRequirementUsage ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Count(graph, "a sysml:SatisfyRequirementUsage ;") != 1 {
		t.Errorf("only the `satisfy` is a SatisfyRequirementUsage:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{"verify r;\n", "verify requirement v : Q;\n", "verify r redefines req;\n", "verify requirement subsets r;\n", "satisfy q by s;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "satisfy r") || strings.Contains(notation, "satisfy requirement") ||
		strings.Contains(notation, "verify requirement subsets r redefines") {
		t.Errorf("`verify r` came back as a satisfy:\n%s", notation)
	}
}
