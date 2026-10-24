package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSelfCheckReadsRelationshipValuedFeatures pins that a house rule may read
// the relationship-valued features of the metamodel (ownedRelationship,
// ownedAnnotation, ownedMembership) and the requirement properties derived
// through memberships (text, requiredConstraint, verifiedRequirement): every
// rule evaluates, and the violations are the ones the model writes.
func TestSelfCheckReadsRelationshipValuedFeatures(t *testing.T) {
	path := filepath.Join("testdata", "self_check_relationship_features.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{{Name: path, Text: string(src)}})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("model did not load cleanly: %v", errs)
	}
	verdicts, stats := s.selfCheckWithCounts("Rules", true)
	if stats.Unevaluated != 0 || stats.EvaluationErrors != 0 {
		t.Fatalf("self-check counts = %+v, want every rule evaluated: %+v", stats, verdicts)
	}
	// The objective of MassTest and its `verify massReq;` are requirement usages
	// of their own (ObjectiveMembership, RequirementVerificationMembership) and
	// write neither text nor a require constraint, so they fail those rules too.
	want := map[string]bool{
		"Rules::requirementHasText fails for Mission::textless":                        false,
		"Rules::requirementHasRequiredConstraint fails for Mission::textless":          false,
		"Rules::requirementHasText fails for Mission::MassTest::":                      false,
		"Rules::requirementHasRequiredConstraint fails for Mission::MassTest::":        false,
		"Rules::requirementHasText fails for Mission::MassTest::massReq":               false,
		"Rules::requirementHasRequiredConstraint fails for Mission::MassTest::massReq": false,
		"Rules::verificationVerifiesSomething fails for Mission::Idle":                 false,
	}
	for _, verdict := range verdicts {
		if verdict.Status != VerdictFails || verdict.Subject == "Self-model check" {
			continue
		}
		key := strings.Replace(verdict.Subject, " for ", " fails for ", 1)
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected violation: %s", verdict.Subject)
			continue
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Errorf("no violation %q in %+v", key, verdicts)
		}
	}
	if stats.Violations != len(want) {
		t.Errorf("violations = %d, want %d: %+v", stats.Violations, len(want), verdicts)
	}
}
