package repl

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSelfCheckAppliesValidationLibraryToWorkspaceElements(t *testing.T) {
	var files []SourceFile
	for _, name := range []string{"self_model_m.sysml", "self_model_n.sysml"} {
		text, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, SourceFile{Name: name, Text: string(text)})
	}
	s := NewSession()
	result := s.SubmitFiles(files)
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixtures did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixtures have model errors")
	}

	verdicts := s.SelfCheck()
	if len(verdicts) == 0 || verdicts[len(verdicts)-1].Status != VerdictHolds {
		t.Fatalf("SelfCheck() = %+v, want a passing summary", verdicts)
	}
	counts := strings.Join(verdicts[len(verdicts)-1].Lines, "\n")
	if !strings.Contains(counts, "Self-model check: ") || strings.Contains(counts, ", 0 violations,") == false {
		t.Fatalf("summary = %q, want zero violations", counts)
	}

	_, stats := s.selfCheckWithCounts("SysMLValidation", false)
	for constraint, want := range map[string]int{
		"validateControlNodeIsComposite":                6,
		"validateEventOccurrenceUsageIsReference":       2,
		"validateEnumerationDefinitionIsVariation":      1,
		"validatePortDefinitionOwnedUsagesNotComposite": 2,
		"validateAttributeUsageIsReference":             7,
	} {
		got := 0
		for name, count := range stats.applications {
			if strings.HasSuffix(name, "::"+constraint) {
				got = count
			}
		}
		if got != want {
			t.Errorf("%s applications = %d, want %d", constraint, got, want)
		}
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("clean fixture produced %d violations and %d evaluation errors",
			stats.violations, stats.evaluationErrors)
	}
}

func TestSelfCheckReportsViolationsAndUnevaluatedConstraints(t *testing.T) {
	const src = `
		package T {
			private import ControlFunctions::*;
			private import SequenceFunctions::*;
			constraint def rejectCompositeFeatures {
				in element : KerML::Feature;
				not element.isComposite;
			}
			constraint def inspectTypeSpecializations {
				in element : KerML::Type;
				element.ownedSpecialization->isEmpty();
			}
			part def P { part c; }
		}
	`
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{{Name: "self_check.sysml", Text: src}})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("model did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("model has errors")
	}

	verdicts, stats := s.selfCheckWithCounts("T", true)
	if stats.violations == 0 {
		t.Fatal("false constraint produced no violation verdicts")
	}
	if stats.unevaluated == 0 {
		t.Fatal("unsupported reflective feature was not counted as unevaluated")
	}
	if stats.evaluationErrors != 0 {
		t.Fatalf("self-check had %d evaluation errors: %+v", stats.evaluationErrors, verdicts)
	}
	foundViolation, foundUnevaluated := false, false
	for _, verdict := range verdicts {
		joined := strings.Join(verdict.Lines, "\n")
		if verdict.Status == VerdictFails && strings.Contains(joined, "rejectCompositeFeatures") &&
			strings.Contains(joined, "T::P::c") && strings.Contains(joined, "self_check.sysml:") {
			foundViolation = true
		}
		if verdict.Status == VerdictUnresolved && strings.Contains(joined, "could not be evaluated") {
			foundUnevaluated = true
		}
	}
	if !foundViolation {
		t.Fatalf("no source-located violation for composite feature: %+v", verdicts)
	}
	if !foundUnevaluated {
		t.Fatalf("no unresolved unevaluated verdict: %+v", verdicts)
	}
	summary := verdicts[len(verdicts)-1]
	if summary.Status != VerdictFails ||
		!strings.Contains(strings.Join(summary.Lines, "\n"), ", "+strconv.Itoa(stats.unevaluated)+" unevaluated") {
		t.Fatalf("summary with a violation and unevaluated checks = %+v, want a failing summary", summary)
	}
}

func TestSelfCheckUnevaluatedOnlyDoesNotFailSummary(t *testing.T) {
	const src = `
		package T {
			private import SequenceFunctions::*;
			constraint def inspectTypeSpecializations {
				in element : KerML::Type;
				element.ownedSpecialization->isEmpty();
			}
			part def P;
		}
	`
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{{Name: "unevaluated_self_check.sysml", Text: src}})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("model did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("model has errors")
	}

	verdicts, stats := s.selfCheckWithCounts("T", true)
	if stats.violations != 0 || stats.evaluationErrors != 0 || stats.unevaluated == 0 {
		t.Fatalf("self-check counts = %+v, want unevaluated outcomes only", stats)
	}
	foundUnevaluated := false
	for _, verdict := range verdicts {
		if verdict.Status == VerdictUnresolved && strings.Contains(strings.Join(verdict.Lines, "\n"), "could not be evaluated") {
			foundUnevaluated = true
			break
		}
	}
	if !foundUnevaluated {
		t.Fatalf("no unresolved unevaluated verdict: %+v", verdicts)
	}
	summary := verdicts[len(verdicts)-1]
	if summary.Status != VerdictHolds ||
		!strings.Contains(strings.Join(summary.Lines, "\n"), ", "+strconv.Itoa(stats.unevaluated)+" unevaluated") {
		t.Fatalf("unevaluated-only summary = %+v, want a passing summary with its unevaluated count", summary)
	}
}

func TestSelfCheckRuntimeTypeErrorMakesSummaryUnresolved(t *testing.T) {
	const src = `
		package T {
			constraint def invalidBooleanResult {
				in element : KerML::Type;
				1;
			}
			part def P;
		}
	`
	s := NewSession()
	s.SubmitFiles([]SourceFile{{Name: "type_error_self_check.sysml", Text: src}})

	verdicts, stats := s.selfCheckWithCounts("T", true)
	if stats.checks == 0 || stats.violations != 0 || stats.unevaluated != 0 || stats.evaluationErrors == 0 {
		t.Fatalf("self-check counts = %+v, want evaluation errors only", stats)
	}
	foundError := false
	for _, verdict := range verdicts[:len(verdicts)-1] {
		joined := strings.Join(verdict.Lines, "\n")
		if verdict.Status == VerdictUnresolved && strings.Contains(joined, "error: could not be evaluated:") &&
			strings.Contains(joined, "Boolean") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatalf("no unresolved evaluation-error verdict: %+v", verdicts)
	}
	summary := verdicts[len(verdicts)-1]
	if summary.Status != VerdictUnresolved {
		t.Fatalf("evaluation-error summary = %+v, want unresolved", summary)
	}
}

func TestSelfCheckReportsNestedCompositePortUsageViolation(t *testing.T) {
	s := loadSelfCheckRepoFixture(t, "tools/referee/reject/testdata/negative/semantic/s25-port-nested-composite-part.sysml")
	verdicts := s.SelfCheck()
	assertSelfCheckViolation(t, verdicts,
		"validatePortUsageNestedUsagesNotComposite",
		"S25PortNestedCompositePart::p::pt")
}

func TestSelfCheckReportsCompositePortDefinitionUsageViolation(t *testing.T) {
	s := loadSelfCheckRepoFixture(t, "tools/referee/reject/testdata/negative/xpect/p26-port-def-nonreferential-usage.sysml")
	verdicts := s.SelfCheck()
	assertSelfCheckViolation(t, verdicts,
		"validatePortDefinitionOwnedUsagesNotComposite",
		"P26PortDefNonReferentialUsage::pd1")
}

func TestSelfCheckIgnoresWorkspaceSysMLValidationPackage(t *testing.T) {
	const src = `
		package SysMLValidation {
			constraint def Reject {
				in element : KerML::Feature;
				false;
			}
		}
		package P { part def T; }
	`
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{{Name: "workspace_validation.sysml", Text: src}})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("model did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("model has errors")
	}

	verdicts := s.SelfCheck()
	if len(verdicts) == 0 || verdicts[len(verdicts)-1].Status != VerdictHolds {
		t.Fatalf("SelfCheck() = %+v, want bundled checks to pass", verdicts)
	}
	summary := strings.Join(verdicts[len(verdicts)-1].Lines, "\n")
	if !strings.Contains(summary, ", 0 violations,") || strings.Contains(summary, ", 0 checks,") {
		t.Fatalf("bundled validation constraints were not applied: %q", summary)
	}
	for _, verdict := range verdicts {
		if strings.Contains(verdict.Subject, "Reject") {
			t.Fatalf("workspace constraint contaminated SelfCheck(): %+v", verdict)
		}
	}
}

func loadSelfCheckRepoFixture(t *testing.T, path string) *Session {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "..", "..", path))
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	s.SubmitFiles([]SourceFile{{Name: filepath.Base(path), Text: string(text)}})
	return s
}

func assertSelfCheckViolation(t *testing.T, verdicts []Verdict, constraint, element string) {
	t.Helper()
	for _, verdict := range verdicts {
		text := verdict.Subject + "\n" + strings.Join(verdict.Lines, "\n")
		if verdict.Status == VerdictFails &&
			strings.Contains(text, constraint) &&
			strings.Contains(text, element) {
			return
		}
	}
	t.Fatalf("no %s violation for %s in %+v", constraint, element, verdicts)
}
