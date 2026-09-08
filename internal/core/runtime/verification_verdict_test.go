package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// verificationVerdictModel writes the four verdicts a body can produce: the
// library's PassIf answering pass and fail, a body binding a literal, a body
// binding none, and one whose subject nothing binds.
const verificationVerdictModel = `
package test {
	part def V { m : ScalarValues::Integer = 0; }
	part zero : V;
	part one : V { m = 1; }

	requirement def R { doc /* zero mass */ }
	requirement r : R;

	verification def Case {
		subject v : V;
		objective { verify requirement : R; }
		VerificationCases::PassIf(v.m == 0)
	}
	verification passing : Case { subject v = zero; }
	verification failing : Case { subject v = one; }

	verification def Silent {
		subject v : V;
		objective { verify r; }
		action step;
	}
	verification silent : Silent { subject v = zero; }

	verification def Stated {
		subject v : V;
		return verdict : VerificationCases::VerdictKind = VerificationCases::VerdictKind::pass;
	}
	verification stated : Stated { subject v = zero; }

	requirement def Q { doc /* unit mass */ }
	requirement q : Q;

	verification def Aux {
		subject v : V;
		out attribute noted : VerificationCases::VerdictKind = VerificationCases::VerdictKind::fail;
		return verdict : VerificationCases::VerdictKind = VerificationCases::VerdictKind::pass;
	}
	verification aux : Aux { subject v = zero; }

	verification def Retargeted {
		subject v : V;
		objective check { verify r; }
	}
	verification retargeted : Retargeted {
		subject v = zero;
		objective check { verify q; }
	}

	verification def Plan {
		subject v : V;
		objective { verify r; }
		verification sub : Case;
	}
	verification plan : Plan { subject v = one; }
}
`

// TestVerificationBodyVerdicts pins the verdict each body shape produces: the
// library's PassIf decides pass and fail, a bound literal is reported as it is,
// a body binding no verdict is inconclusive, and a body that cannot run is an
// error verdict carrying the message.
func TestVerificationBodyVerdicts(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationVerdictModel))
	for _, tc := range []struct {
		fqn    string
		kind   VerdictKind
		detail string
	}{
		{fqn: "test::passing", kind: VerdictPass},
		{fqn: "test::failing", kind: VerdictFail},
		{fqn: "test::silent", kind: VerdictInconclusive, detail: "no VerdictKind"},
		{fqn: "test::stated", kind: VerdictPass},
		{fqn: "test::aux", kind: VerdictPass},
		{fqn: "test::Case", kind: VerdictError, detail: "subject is unbound"},
	} {
		result, err := ctx.RunVerification(oneSymbol(t, idx, tc.fqn), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunVerification(%s): %v", tc.fqn, err)
		}
		if result.Verdict.Kind != tc.kind {
			t.Errorf("%s: verdict = %+v, want %s", tc.fqn, result.Verdict, tc.kind)
		}
		if tc.detail != "" && !strings.Contains(result.Verdict.Detail, tc.detail) {
			t.Errorf("%s: detail %q does not report %q", tc.fqn, result.Verdict.Detail, tc.detail)
		}
		if result.Verdict.Case != tc.fqn {
			t.Errorf("%s: case = %q", tc.fqn, result.Verdict.Case)
		}
		if tc.detail == "" && result.Verdict.Detail != "" {
			t.Errorf("%s: decided verdict carries detail %q", tc.fqn, result.Verdict.Detail)
		}
	}
}

// TestVerificationSubcaseVerdictsAreReportedOnTheirOwn pins that a case
// performing a subcase reports the subcase's verdict beside its own, read from
// the same run, since the library states no roll-up.
func TestVerificationSubcaseVerdictsAreReportedOnTheirOwn(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationVerdictModel))
	result, err := ctx.RunVerification(oneSymbol(t, idx, "test::plan"), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatalf("RunVerification(test::plan): %v", err)
	}
	if result.Verdict.Kind != VerdictInconclusive {
		t.Errorf("plan verdict = %+v, want inconclusive", result.Verdict)
	}
	if len(result.Subcases) != 1 || result.Subcases[0].Kind != VerdictFail {
		t.Fatalf("subcases = %+v, want one failing", result.Subcases)
	}
	if !strings.HasSuffix(result.Subcases[0].Case, "sub") {
		t.Errorf("subcase case = %q, want the performed subcase", result.Subcases[0].Case)
	}
}

// TestVerificationVerdictsForRequirement pins what a reporting surface asks
// for: the body verdict of each case usage verifying a requirement, its
// subcases beside it, a case nested in another reported only as that subcase.
// A case is reported once however many of the searched scopes reach it.
func TestVerificationVerdictsForRequirement(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationVerdictModel))
	root := idx.DocumentRoot("<test>")
	var got []string
	scopes := []*symbols.Scope{root, root}
	for _, v := range ctx.VerificationVerdictsIn(scopes, oneSymbol(t, idx, "test::R")) {
		got = append(got, v.Case+"="+string(v.Kind))
	}
	want := []string{"test::passing=pass", "test::failing=fail"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("VerificationVerdictsIn(test::R) = %v, want %v", got, want)
	}
}

// TestVerificationsOfRequirement pins the verification cases a requirement is
// verified by: named directly (`verify r`) or by its definition
// (`verify requirement : R`), through a case usage's inherited objective. A
// usage redeclaring an inherited objective verifies what it says, not also what
// the objective it restates said.
func TestVerificationsOfRequirement(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationVerdictModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		requirement string
		want        []string
	}{
		{requirement: "test::R", want: []string{"test::Case", "test::passing", "test::failing", "test::Plan::sub"}},
		{requirement: "test::r", want: []string{"test::Silent", "test::silent", "test::Retargeted", "test::Plan", "test::plan"}},
		{requirement: "test::q", want: []string{"test::retargeted"}},
	} {
		var got []string
		for _, sym := range ctx.VerificationsOf(root, oneSymbol(t, idx, tc.requirement)) {
			got = append(got, ctx.qualifiedSymbolName(sym))
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("VerificationsOf(%s) = %v, want %v", tc.requirement, got, tc.want)
		}
	}
}
