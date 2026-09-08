package repl

import (
	"strings"
	"testing"
)

// TestRequirementReportsTheVerificationBodyVerdicts checks that %requirement
// keeps the satisfaction verdict it reported before and adds the verdict the
// body of every case verifying that requirement produced.
func TestRequirementReportsTheVerificationBodyVerdicts(t *testing.T) {
	s := loadFixture(t, "testdata/verification_verdicts.sysml")
	out := run(t, s, "%requirement Landing::touchdown")
	wants(t, out,
		"✓ Requirement Landing::touchdown satisfied",
		"✓ Verification Landing::checkSlow verdict: pass",
		"✗ Verification Landing::checkFast verdict: fail",
		"? Verification Landing::silentCheck verdict: inconclusive",
	)

	verdict := s.CheckRequirement("Landing::touchdown")
	if verdict.Status != VerdictHolds {
		t.Errorf("status = %v, want the requirement satisfied", verdict.Status)
	}
	var got []string
	for _, v := range verdict.Verifications {
		got = append(got, v.Case+"="+v.Kind)
	}
	want := "Landing::checkSlow=pass Landing::checkFast=fail Landing::silentCheck=inconclusive"
	if joined := join(got); joined != want {
		t.Errorf("body verdicts = %q, want %q", joined, want)
	}
}

// TestSatisfyReportsTheVerificationBodyVerdicts checks the same beside the
// verdict of a satisfaction assertion.
func TestSatisfyReportsTheVerificationBodyVerdicts(t *testing.T) {
	s := loadFixture(t, "testdata/verification_verdicts.sysml")
	wants(t, run(t, s, "%satisfy"),
		"✓ satisfy touchdown by slowLander holds",
		"✓ Verification Landing::checkSlow verdict: pass",
		"✗ Verification Landing::checkFast verdict: fail",
	)
}

// TestSatisfyOfADeclaredRequirementReportsTheBodyVerdicts checks that a
// satisfaction stating its own requirement (`satisfy requirement r by p`)
// reports the cases verifying that requirement beside its own verdict.
func TestSatisfyOfADeclaredRequirementReportsTheBodyVerdicts(t *testing.T) {
	s := loadFixture(t, "testdata/verification_verdicts_declared.sysml")
	lines := strings.Split(run(t, s, "%satisfy"), "\n")
	want := "✓ Verification Landing::checkSlow verdict: pass"
	for i, line := range lines {
		if !strings.Contains(line, "satisfy grounded by slowLander holds") {
			continue
		}
		if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) != want {
			t.Fatalf("the declared satisfaction reports %q, want %q beside it",
				strings.Join(lines[i:], " | "), want)
		}
		return
	}
	t.Fatalf("output does not report the declared satisfaction:\n%s", strings.Join(lines, "\n"))
}

// TestAnalysisRunsAVerificationCase checks that %analysis runs a verification
// case rather than refusing it, reporting the verdict of its body.
func TestAnalysisRunsAVerificationCase(t *testing.T) {
	s := loadFixture(t, "testdata/verification_verdicts.sysml")
	wants(t, run(t, s, "%analysis Landing::checkFast"),
		"✗ Verification Landing::checkFast verdict: fail")
	wants(t, run(t, s, "%analysis Landing::checkSlow"),
		"✓ Verification Landing::checkSlow verdict: pass")
	// A definition binds no subject, so its body cannot run: that is the
	// case's error verdict, carrying the reason.
	wants(t, run(t, s, "%analysis Landing::SpeedCheck"), "subject")
}

// TestAnalysisKeepsACaseStatusApartFromItsSubcases checks that a case whose own
// body passes holds even though a subcase it performed failed: the library
// states no roll-up, so the subcase answers for itself alone.
func TestAnalysisKeepsACaseStatusApartFromItsSubcases(t *testing.T) {
	s := loadFixture(t, "testdata/verification_verdicts.sysml")
	wants(t, run(t, s, "%analysis Landing::checkedPlan"),
		"✓ Verification Landing::checkedPlan verdict: pass",
		"✗ Verification Landing::CheckedPlan::checkOne verdict: fail (subcase)",
	)
	if got := s.RunAnalysis("Landing::checkedPlan"); got.Status != VerdictHolds {
		t.Errorf("status = %v, want the case's own passing verdict", got.Status)
	}
}

// join renders verdict names for one comparison.
func join(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += " "
		}
		out += part
	}
	return out
}
