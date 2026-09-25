package xpect

import "testing"

// A wording-only row must show the same rule about the same element. Severity
// and offset are checked by the caller and are deliberately not enough.
func TestWordingOnlyRequiresRuleAndElementIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, declared, ours, class string
		want                        bool
	}{{
		name:     "same reference",
		declared: "Couldn't resolve reference to Classifier 'A::a1'.",
		ours:     "unresolved reference: A::a1",
		class:    "unresolved-reference",
		want:     true,
	}, {
		name:     "our suggestion does not change the rule",
		declared: "Couldn't resolve reference to Feature 'a1'.",
		ours:     "unresolved reference: a1 — did you mean OuterPackage::A::a1?",
		class:    "unresolved-reference",
		want:     true,
	}, {
		name:     "unresolved member names the segment that failed",
		declared: "Couldn't resolve reference to Element 'A::b'.",
		ours:     "unresolved member: b in A",
		class:    "unresolved-reference",
		want:     true,
	}, {
		name:     "another element is not the same finding",
		declared: "Couldn't resolve reference to Classifier 'A::a1'.",
		ours:     "unresolved reference: A::a2",
	}, {
		name:     "a reference that resolves to the wrong kind is another rule",
		declared: "Couldn't resolve reference to Type 'test'.",
		ours:     "type must be a type, found package",
	}, {
		name:     "a parse error and a visibility rule are not one rule",
		declared: "mismatched input 'import' expecting '}'",
		ours:     "import without a visibility indicator: SysML v2 requires public, private or protected before 'import'",
	}, {
		name:     "a suffix match is not admitted where we name the whole reference",
		declared: "Couldn't resolve reference to Classifier 'A::a1'.",
		ours:     "unresolved reference: a1",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			class, ok := wordingOnly(tc.declared, tc.ours)
			if ok != tc.want || class != tc.class {
				t.Errorf("wordingOnly = %q, %v; want %q, %v", class, ok, tc.class, tc.want)
			}
		})
	}
}

// A wording-only row is counted inside agreement, and the sub-count says how
// much of the agreement it is.
func TestWordingOnlyCountsInsideAgreement(t *testing.T) {
	report := Report{Suites: []SuiteReport{{Name: "kerml", Files: []fileResult{{
		Path: "a.kerml.xt",
		Rows: []row{
			{Kind: kindErrors, Line: 1, Verdict: verdictAgree},
			{Kind: kindErrors, Line: 2, Verdict: verdictWordingOnly, Rule: "unresolved-reference"},
			{Kind: kindErrors, Line: 3, Verdict: verdictDisagree, Tolerance: toleranceMessage},
		},
	}}}}}
	report.summarize()
	if got := report.Totals; got.Agree != 2 || got.WordingOnly != 1 || got.Disagree != 1 {
		t.Errorf("totals agree/wordingOnly/disagree = %d/%d/%d, want 2/1/1", got.Agree, got.WordingOnly, got.Disagree)
	}
	kind := report.Kinds[0]
	if kind.Agree != 2 || kind.WordingOnly != 1 || kind.SameLocation != 1 {
		t.Errorf("errors agree/wordingOnly/sameLocation = %d/%d/%d, want 2/1/1",
			kind.Agree, kind.WordingOnly, kind.SameLocation)
	}
	if got := report.Suites[0].Files[0].Agree; got != 2 {
		t.Errorf("file agree = %d, want 2", got)
	}
}
