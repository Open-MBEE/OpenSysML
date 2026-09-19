package passes

import "testing"

// The reference reports `Must invoke a behavior or a behavioral feature` for a
// non-behavior definition and for a feature typed by one, at the target's
// location; both are matched here.
func TestW7GInvokingANonBehaviourIsReportedAsTheReferenceWordsIt(t *testing.T) {
	const src = `package C {
		part def P;
		attribute b = P(1);
		attribute def D;
		attribute d : D;
		attribute e = d(1);
	}`
	diags := only(f23AllDiags(t, src), "invocation-not-behavior")
	if len(diags) != 2 {
		t.Fatalf("expected both non-behavior invocations to be reported, got %v", diags)
	}
	for _, d := range diags {
		if d.Message != "Must invoke a behavior or a behavioral feature" {
			t.Fatalf("expected the reference's wording, got %q", d.Message)
		}
	}
}

// A name two imports bring in is judged over every candidate: a call selects the
// behavioral one, and is reported once when there is none.
func TestW7GAmbiguousAndValidInvocationTargets(t *testing.T) {
	const mixed = `package A1 { part def N; }
	package A2 { action def N; }
	package C {
		private import A1::*;
		private import A2::*;
		attribute x = N(1);
	}`
	if diags := only(f23AllDiags(t, mixed), "invocation-not-behavior"); len(diags) != 0 {
		t.Fatalf("expected the behavioral candidate to be selected, got %v", diags)
	}
	const noBehavior = `package A1 { part def N; }
	package A2 { attribute def N; }
	package C {
		private import A1::*;
		private import A2::*;
		attribute x = N(1);
	}`
	if diags := only(f23AllDiags(t, noBehavior), "invocation-not-behavior"); len(diags) != 1 {
		t.Fatalf("expected the non-behavior to be reported once, got %v", diags)
	}
	const valid = `package V {
		calc def F { in p; }
		action def G { in q; }
		attribute y = F(1);
		attribute z = G(1);
	}`
	if diags := only(f23AllDiags(t, valid), "invocation-not-behavior"); len(diags) != 0 {
		t.Fatalf("a behavioral target is invocable, got %v", diags)
	}
}

// The reference reports the rule on an unresolved target too; our type tier is
// skipped for a name that did not resolve, so only the resolution error stands.
func TestW7GUnresolvedInvocationTargetIsLeftToNameResolution(t *testing.T) {
	const src = `package C { attribute x = Missing(1); }`
	if diags := only(f23AllDiags(t, src), "invocation-not-behavior"); len(diags) != 0 {
		t.Fatalf("an unresolved target is reported by name resolution alone, got %v", diags)
	}
}
