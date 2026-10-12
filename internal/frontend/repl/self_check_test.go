package repl

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	if len(verdicts) == 0 {
		t.Fatal("SelfCheck() returned no summary")
	}
	counts := strings.Join(verdicts[len(verdicts)-1].Lines, "\n")
	if !strings.Contains(counts, "Self-model check: ") {
		t.Fatalf("summary = %q, want the self-model check summary", counts)
	}
	for _, verdict := range verdicts {
		if verdict.Status == VerdictFails &&
			strings.Contains(verdict.Subject, "validateDefinitionVariationIsAbstract") &&
			strings.Contains(verdict.Subject, "M::Color") {
			t.Fatalf("variation definition is no longer abstract: %+v", verdict)
		}
	}
	for _, verdict := range verdicts {
		if strings.HasPrefix(verdict.Subject, "SysMLValidation::validatePortUsageIsReference for M::P::pp") {
			t.Errorf("referential port usage reported a violation: %+v", verdict)
		}
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
	if stats.violations != 0 {
		t.Errorf("fixture produced %d violations, want variations to be abstract", stats.violations)
	}
	if stats.evaluationErrors != 0 {
		t.Errorf("the corrected batch-2 constraints had %d evaluation errors", stats.evaluationErrors)
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
				element.ownedRelationship->isEmpty();
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
				element.ownedRelationship->isEmpty();
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

func TestSelfCheckBatch2ConstraintFixtures(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		positive   SourceFile
		negative   SourceFile
	}{
		{
			name:       "binding connector is binary",
			constraint: "validateBindingConnectorIsBinary",
			positive: SourceFile{Name: "binary.kerml", Text: `package T {
				class A {
					feature a; feature b;
					binding a = b;
				}
			}`},
			negative: selfCheckFixtureFile(t, "tools/referee/reject/testdata/negative/semantic/k49-binding-connector-with-three-ends.kerml"),
		},
		{
			name:       "binding connector with repeated feature is binary",
			constraint: "validateBindingConnectorIsBinary",
			positive: SourceFile{Name: "repeated_binding.kerml", Text: `package T {
				class A {
					feature a;
					binding repeated of a = a;
				}
			}`},
			negative: selfCheckFixtureFile(t, "tools/referee/reject/testdata/negative/semantic/k49-binding-connector-with-three-ends.kerml"),
		},
		{
			name:       "case subject is first input",
			constraint: "validateCaseDefinitionSubjectParameterPosition",
			positive: SourceFile{Name: "case_positive.sysml", Text: `package T {
				part def P;
				case def CD { subject s : P; in x : P; }
			}`},
			negative: selfCheckFixtureFile(t, "tools/referee/reject/testdata/negative/semantic/s10-case-def-subject-not-first.sysml"),
		},
		{
			name:       "occurrence usage has at most one individual definition",
			constraint: "validateOccurrenceUsageIndividualDefinition",
			positive: SourceFile{Name: "individual_definition_positive.sysml", Text: `package T {
				individual occurrence def I1;
				individual occurrence def I2;
				occurrence u : I1;
			}`},
			negative: SourceFile{Name: "individual_definition_negative.sysml", Text: `package T {
				individual occurrence def I1;
				individual occurrence def I2;
				occurrence u : I1, I2;
			}`},
		},
		{
			name:       "portion usage has occurrence owner",
			constraint: "validateOccurrenceUsagePortionKind",
			positive: SourceFile{Name: "portion_positive.sysml", Text: `package T {
				occurrence def Timeline {
					snapshot occurrence point;
					timeslice occurrence interval;
				}
			}`},
			negative: SourceFile{Name: "portion_negative.sysml", Text: `package T {
				snapshot occurrence point;
			}`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, fixture := range []struct {
				name     string
				file     SourceFile
				wantFail bool
			}{
				{name: "positive", file: tt.positive},
				{name: "negative", file: tt.negative, wantFail: true},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					s := NewSession()
					result := s.SubmitFiles([]SourceFile{fixture.file})
					if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 && !fixture.wantFail {
						t.Fatalf("fixture did not load cleanly: %v", errs)
					}
					if s.HasErrors() && !fixture.wantFail {
						t.Fatal("fixture has model errors")
					}
					verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
					failed := false
					for _, verdict := range verdicts {
						text := verdict.Subject + "\n" + strings.Join(verdict.Lines, "\n")
						if verdict.Status == VerdictFails && strings.Contains(text, tt.constraint) {
							failed = true
						}
					}
					if failed != fixture.wantFail {
						t.Errorf("%s violation = %t, want %t; stats=%+v verdicts=%+v",
							tt.constraint, failed, fixture.wantFail, stats, verdicts)
					}
					if fixture.wantFail && !failed {
						t.Errorf("negative fixture produced no %s violation: %+v", tt.constraint, verdicts)
					}
				})
			}
		})
	}
}

func TestSelfCheckRequirementSubjectInputOrdering(t *testing.T) {
	const src = `package T {
		part def Subject;
		part owner : Subject;
		concern def MassBudget {
			subject robot : Subject;
			in budget : Subject;
		}
		viewpoint def Perspective {
			subject system : Subject;
			in viewpointInput : Subject;
			frame concern framed : MassBudget;
		}
		requirement def Base {
			subject s : Subject;
			in inherited : Subject;
			in inheritedTail : Subject;
		}
		requirement req : Base {
			subject = owner;
			in ownInput : Subject;
		}
		verification def Verifier {
			subject system : Subject;
			objective { verify req; }
		}
		concern concernUse : MassBudget {
			subject = owner;
			in ownInput : Subject;
		}
		viewpoint viewpointUse : Perspective {
			subject = owner;
			in ownInput : Subject;
		}
	}`
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{{Name: "requirement_subjects.sysml", Text: src}})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("model did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("model has errors")
	}

	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.checks == 0 || stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Fatalf("self-check counts = %+v; verdicts = %+v", stats, verdicts)
	}
}

func TestSelfCheckPartUsageDefinitionForObjectivesAndVariants(t *testing.T) {
	file := SourceFile{Name: "reflective_part_usage.sysml", Text: `package P {
		requirement def Requirement { subject subject; }
		analysis def Analysis { objective goal : Requirement; }
		part def Base;
		part selected : Base;
		variation part def Choices :> Base { variant selected; }
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}

	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.checks == 0 || stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Fatalf("self-check counts = %+v; verdicts = %+v", stats, verdicts)
	}
	if got := stats.applications["SysMLValidation::validatePartUsagePartDefinition"]; got != 2 {
		t.Errorf("part-definition constraint applications = %d, want 2 (subject and part, not objective or variant reference)", got)
	}
}

func TestSelfCheckMetadataFeatureMetaclass(t *testing.T) {
	file := SourceFile{Name: "metadata_feature.sysml", Text: `package P {
		private import ScalarValues::*;
		metadata def Marker { attribute label : String; }
		part def Base;
		part actual : Base {
			@Marker { label = "recorded"; }
		}
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}

	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.applications["SysMLValidation::validateMetadataFeatureMetaclass"] == 0 {
		t.Fatal("SelfCheck() did not apply validateMetadataFeatureMetaclass")
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("metadata feature self-check had %d violations and %d errors: %+v", stats.violations, stats.evaluationErrors, verdicts)
	}
}

func TestSelfCheckVariationDefinitionsAndUsagesAreAbstract(t *testing.T) {
	file := SourceFile{Name: "variations.sysml", Text: `package T {
		part def Base;
		variation part def Choices :> Base { variant part small : Base; }
		part def Owner {
			variation part choice : Base { variant part small : Base; }
		}
		enum def Color { red; green; }
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}
	_, stats := s.selfCheckWithCounts("SysMLValidation", true)
	for _, constraint := range []string{
		"validateDefinitionVariationIsAbstract",
		"validateUsageVariationIsAbstract",
	} {
		if stats.applications["SysMLValidation::"+constraint] == 0 {
			t.Errorf("SelfCheck() did not apply %s: %+v", constraint, stats.applications)
		}
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("variation self-check had %d violations and %d errors", stats.violations, stats.evaluationErrors)
	}
}

func TestSelfCheckReferentialUsagesAreNotComposite(t *testing.T) {
	file := SourceFile{Name: "referential.sysml", Text: `package T {
		part def Base;
		action sequential;
		part def Owner {
			in part directed;
			end part endpoint;
		}
		variation part orphan : Base {
			variant part orphanOption : Base;
		}
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}
	_, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.applications["SysMLValidation::validateUsageIsReferential"] == 0 {
		t.Fatal("SelfCheck() did not apply validateUsageIsReferential")
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("referential self-check had %d violations and %d errors", stats.violations, stats.evaluationErrors)
	}
}

func TestSelfCheckConnectionDefinitionsAreSufficient(t *testing.T) {
	file := SourceFile{Name: "connection_definition.sysml", Text: `package T {
		connection def C {
			end item source;
			end item target;
		}
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}
	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.applications["SysMLValidation::validateConnectionDefinitionIsSufficient"] == 0 {
		t.Fatal("SelfCheck() did not apply validateConnectionDefinitionIsSufficient")
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("connection definition self-check had %d violations and %d errors: %+v", stats.violations, stats.evaluationErrors, verdicts)
	}
}

func TestSelfCheckConnectorRelatedFeaturesCoverEndForms(t *testing.T) {
	file := SourceFile{Name: "connector_related.sysml", Text: `package T {
		private import ScalarValues::*;
		private import Flows::Message;
		part def A {
			attribute x : Integer;
			attribute y : Integer;
		}
		part def Host {
			part a : A;
			part b : A;
			connection byPair connect (a, b);
			flow byFlow of Integer from a.x to b.y;
			message byMessage of Integer from a.x to b.y;
			message incoming : Message[*];
			binding byReferences {
				end feature references a;
				end feature references b;
			}
		}
		action def Sequence {
			action firstNode;
			action secondNode;
			succession explicit first firstNode then secondNode;
			first firstNode then secondNode {
				action nested;
			}
			first firstNode;
			then secondNode;
		}
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}

	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.evaluationErrors != 0 {
		t.Fatalf("self-check had %d evaluation errors: %+v", stats.evaluationErrors, verdicts)
	}
	if stats.applications["SysMLValidation::validateConnectorRelatedFeatures"] < 4 {
		t.Fatalf("SelfCheck() did not cover the connector forms: %+v", stats.applications)
	}
	for _, verdict := range verdicts[:len(verdicts)-1] {
		if strings.Contains(strings.Join(verdict.Lines, "\n"), "validateConnectorRelatedFeatures") &&
			verdict.Status != VerdictHolds {
			t.Errorf("connector-related self-check did not hold: %+v", verdict)
		}
	}
}

func TestSelfCheckTopLevelKerMLSuccessionRelatedFeatures(t *testing.T) {
	file := SourceFile{Name: "top_level_succession.kerml", Text: `package T {
		private import ScalarValues::*;
		feature transitionLinkSource[0..1];
		feature trigger[1..*];
		feature triggerNum : Natural[1] = 1;
		succession triggerAfter [triggerNum]
			first [0..1] transitionLinkSource
			then [*] trigger;
	}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}

	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.applications["SysMLValidation::validateConnectorRelatedFeatures"] == 0 {
		t.Fatal("SelfCheck() did not apply validateConnectorRelatedFeatures")
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("top-level succession self-check had %d violations and %d errors: %+v",
			stats.violations, stats.evaluationErrors, verdicts)
	}
}

func selfCheckFixtureFile(t *testing.T, path string) SourceFile {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "..", "..", path))
	if err != nil {
		t.Fatal(err)
	}
	return SourceFile{Name: filepath.Base(path), Text: string(text)}
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

func loadSelfCheckFixture(t *testing.T, files ...SourceFile) *Session {
	t.Helper()
	s := NewSession()
	result := s.SubmitFiles(files)
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixtures did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixtures have model errors")
	}
	return s
}

func TestSelfCheckPackageAppliesARuleThatHolds(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M {
			part def P { doc /* stated */ }
			part def Q { doc /* also stated */ }
		}`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				private import SequenceFunctions::*;
				constraint def partDefinitionHasDocumentation {
					in pd : SysML::PartDefinition;
					not pd.documentation->isEmpty();
				}
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	if got := stats.applications["Acme::Rules::partDefinitionHasDocumentation"]; got == 0 {
		t.Fatalf("rule was not applied: %+v", stats.applications)
	}
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Fatalf("documented part defs produced %d violations and %d errors: %+v", stats.violations, stats.evaluationErrors, verdicts)
	}
}

func TestSelfCheckPackageReportsAViolationUnderItsQualifiedName(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M {
			part def P;
		}`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				private import SequenceFunctions::*;
				constraint def partDefinitionHasDocumentation {
					in pd : SysML::PartDefinition;
					not pd.documentation->isEmpty();
				}
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.violations == 0 {
		t.Fatalf("undocumented part def produced no violation: %+v", verdicts)
	}
	found := false
	for _, verdict := range verdicts {
		text := verdict.Subject + "\n" + strings.Join(verdict.Lines, "\n")
		if verdict.Status == VerdictFails && strings.HasPrefix(verdict.Subject, "Acme::Rules::") &&
			strings.Contains(text, "fails for M::P") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no violation under the rule's qualified name: %+v", verdicts)
	}
}

func TestSelfCheckPackageEvaluationErrorLeavesSummaryUnresolved(t *testing.T) {
	// The division analyses cleanly but fails at runtime, so the run reports an
	// evaluation error rather than a model error the CLI would gate on.
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P; }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				private import SequenceFunctions::*;
				constraint def divideByZero {
					in pd : SysML::PartDefinition;
					pd.name->size() / 0 > 0;
				}
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.evaluationErrors == 0 {
		t.Fatalf("non-Boolean rule produced no evaluation error: %+v", stats)
	}
	if summary := verdicts[len(verdicts)-1]; summary.Status != VerdictUnresolved {
		t.Fatalf("summary = %+v, want unresolved", summary)
	}
}

func TestSelfCheckPackageUnknownNameEvaluatesNothing(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P; }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme { package Rules { } }`})

	verdicts, err := s.SelfCheckPackages([]string{"Acme::Nope", "Acme::AlsoNope"})
	if verdicts != nil {
		t.Fatalf("verdicts = %+v, want nil", verdicts)
	}
	if !errors.Is(err, ErrSelfCheckPackageNotFound) {
		t.Fatalf("err = %v, want ErrSelfCheckPackageNotFound", err)
	}
	for _, name := range []string{"Acme::Nope", "Acme::AlsoNope"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error does not name %s: %v", name, err)
		}
	}
	if err := s.ResolveSelfCheckPackages([]string{"Acme::Nope"}); !errors.Is(err, ErrSelfCheckPackageNotFound) {
		t.Fatalf("ResolveSelfCheckPackages = %v, want ErrSelfCheckPackageNotFound", err)
	}
	if err := s.ResolveSelfCheckPackages([]string{"Acme::Rules"}); err != nil {
		t.Fatalf("ResolveSelfCheckPackages = %v, want nil", err)
	}
}

func TestSelfCheckPackageSkipsANonMetaclassParameterWithAWarning(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P; }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				private import ScalarValues::*;
				private import SequenceFunctions::*;
				constraint def integerRule {
					in x : ScalarValues::Integer;
					true;
				}
				constraint def modelTypeRule {
					in x : M::P;
					true;
				}
				constraint def holdsRule {
					in pd : SysML::PartDefinition;
					true;
				}
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Acme::Rules::integerRule", "Acme::Rules::modelTypeRule"} {
		if stats.applications[name] != 0 {
			t.Errorf("skipped rule %s was applied", name)
		}
		found := false
		for _, verdict := range verdicts {
			text := strings.Join(verdict.Lines, "\n")
			if verdict.Subject == name && strings.Contains(text, "warning:") &&
				strings.Contains(text, "is skipped") {
				found = true
			}
		}
		if !found {
			t.Errorf("no skip warning for %s: %+v", name, verdicts)
		}
	}
	if stats.applications["Acme::Rules::holdsRule"] == 0 {
		t.Error("the metaclass-typed rule was not applied")
	}
}

func TestSelfCheckPackageWarnsWhenAPackageYieldsNoConstraints(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P; }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme { package Rules { part def Z; } }`})

	verdicts, _, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, verdict := range verdicts {
		if strings.Contains(strings.Join(verdict.Lines, "\n"), "has no applicable constraint definitions") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no empty-package warning: %+v", verdicts)
	}
	if summary := verdicts[len(verdicts)-1]; !strings.Contains(strings.Join(summary.Lines, "\n"), "Self-model check:") {
		t.Fatalf("run did not complete: %+v", summary)
	}
}

func TestSelfCheckPackageAppliesNestedPackageConstraints(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				package Nested {
					private import SequenceFunctions::*;
					constraint def partDefinitionHasDocumentation {
						in pd : SysML::PartDefinition;
						not pd.documentation->isEmpty();
					}
				}
			}
		}`})

	_, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	if got := stats.applications["Acme::Rules::Nested::partDefinitionHasDocumentation"]; got == 0 {
		t.Fatalf("nested constraint was not applied: %+v", stats.applications)
	}
}

func TestSelfCheckAppliesSeveralPackagesAtOnce(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		SourceFile{Name: "rules_a.sysml", Text: `package Acme {
			package Rules {
				private import SequenceFunctions::*;
				constraint def documented {
					in pd : SysML::PartDefinition;
					not pd.documentation->isEmpty();
				}
			}
		}`},
		SourceFile{Name: "rules_b.sysml", Text: `package Beta {
			constraint def anything {
				in pd : SysML::PartDefinition;
				true;
			}
		}`})

	_, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules", "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Acme::Rules::documented", "Beta::anything"} {
		if stats.applications[name] == 0 {
			t.Errorf("%s was not applied: %+v", name, stats.applications)
		}
	}
}

func TestSelfCheckPackageElementsAreNotChecked(t *testing.T) {
	const model = `package M { part def P { doc /* stated */ } }`
	const rules = `package Acme {
		package Rules {
			private import SequenceFunctions::*;
			constraint def documented {
				in pd : SysML::PartDefinition;
				not pd.documentation->isEmpty();
			}
		}
	}`
	modelOnly := loadSelfCheckFixture(t, SourceFile{Name: "model.sysml", Text: model})
	_, baseline, err := modelOnly.selfCheckRun("SysMLValidation", false, nil)
	if err != nil {
		t.Fatal(err)
	}

	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: model},
		SourceFile{Name: "rules.sysml", Text: rules})
	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.elements != baseline.elements {
		t.Errorf("elements = %d, want %d as checked of the model alone", stats.elements, baseline.elements)
	}
	for _, verdict := range verdicts {
		if strings.Contains(verdict.Subject, "for Acme::") {
			t.Errorf("a rule-package element was checked: %+v", verdict)
		}
	}
}

func TestSelfCheckIgnoresRulePackagesWithoutTheFlag(t *testing.T) {
	files := []SourceFile{
		{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				constraint def anything {
					in pd : SysML::PartDefinition;
					false;
				}
			}
		}`},
	}
	s := loadSelfCheckFixture(t, files...)
	for _, verdict := range s.SelfCheck() {
		if strings.Contains(verdict.Subject, "Acme::") {
			t.Fatalf("SelfCheck() applied a rule-package constraint: %+v", verdict)
		}
	}
	got, err := s.SelfCheckPackages(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := s.SelfCheck(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SelfCheckPackages(nil) = %+v, want SelfCheck() = %+v", got, want)
	}
}

func TestSelfCheckPackageNamingTheBundledPackageDoesNotDoubleApply(t *testing.T) {
	s := loadSelfCheckFixture(t, SourceFile{Name: "model.sysml", Text: `package M { part def P; }`})

	_, bundled, err := s.selfCheckRun("SysMLValidation", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	verdicts, doubled, err := s.selfCheckRun("SysMLValidation", false, []string{"SysMLValidation"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range bundled.applications {
		if doubled.applications[name] != want {
			t.Errorf("%s applications = %d, want %d", name, doubled.applications[name], want)
		}
	}
	for _, verdict := range verdicts {
		if strings.Contains(strings.Join(verdict.Lines, "\n"), "has no applicable constraint definitions") {
			t.Errorf("SysMLValidation warned as empty: %+v", verdict)
		}
	}
}

func TestSelfCheckPackageReopenedAcrossFilesAppliesOnce(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		SourceFile{Name: "rules_a.sysml", Text: `package Acme {
			package Rules {
				private import SequenceFunctions::*;
				constraint def documented {
					in pd : SysML::PartDefinition;
					not pd.documentation->isEmpty();
				}
			}
		}`},
		SourceFile{Name: "rules_b.sysml", Text: `package Acme {
			package Rules {
				part def Stub;
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::Rules"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.applications["Acme::Rules::documented"] == 0 {
		t.Fatalf("re-opened rule was not applied: %+v", stats.applications)
	}
	for _, verdict := range verdicts {
		if strings.Contains(strings.Join(verdict.Lines, "\n"), "has no applicable constraint definitions") {
			t.Errorf("one declaration being empty warned the whole package: %+v", verdict)
		}
	}
}

func TestSelfCheckPackageParentAndNestedBothRequestedWarnForNeither(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package Rules {
				package Nested {
					private import SequenceFunctions::*;
					constraint def documented {
						in pd : SysML::PartDefinition;
						not pd.documentation->isEmpty();
					}
				}
			}
		}`})

	verdicts, stats, err := s.selfCheckRun("SysMLValidation", false,
		[]string{"Acme::Rules", "Acme::Rules::Nested"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.applications["Acme::Rules::Nested::documented"] == 0 {
		t.Fatalf("nested rule was not applied: %+v", stats.applications)
	}
	for _, verdict := range verdicts {
		if strings.Contains(strings.Join(verdict.Lines, "\n"), "has no applicable constraint definitions") {
			t.Errorf("a requested package warned as empty: %+v", verdict)
		}
	}
}

func TestSelfCheckPackageQuotedSegmentsResolve(t *testing.T) {
	s := loadSelfCheckFixture(t,
		SourceFile{Name: "model.sysml", Text: `package M { part def P { doc /* stated */ } }`},
		SourceFile{Name: "rules.sysml", Text: `package Acme {
			package 'Modeling Rules' {
				private import SequenceFunctions::*;
				constraint def documented {
					in pd : SysML::PartDefinition;
					not pd.documentation->isEmpty();
				}
			}
		}`})

	_, stats, err := s.selfCheckRun("SysMLValidation", false, []string{"Acme::'Modeling Rules'"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.applications["Acme::Modeling Rules::documented"] == 0 {
		t.Fatalf("quoted rule was not applied: %+v", stats.applications)
	}
}

func TestSelfCheckPackageQuotedSegmentHoldingColonsStaysDistinct(t *testing.T) {
	files := []SourceFile{
		{Name: "model.sysml", Text: `package M { part def P; }`},
		{Name: "rules.sysml", Text: `package 'A::' {
			constraint def anything {
				in pd : SysML::PartDefinition;
				true;
			}
		}`},
	}
	for _, order := range [][]string{{"'A::'", "A::"}, {"A::", "'A::'"}} {
		s := loadSelfCheckFixture(t, files...)
		verdicts, err := s.SelfCheckPackages(order)
		if verdicts != nil {
			t.Fatalf("%v: verdicts = %+v, want nil", order, verdicts)
		}
		if !errors.Is(err, ErrSelfCheckPackageNotFound) {
			t.Fatalf("%v: err = %v, want ErrSelfCheckPackageNotFound", order, err)
		}
		if !strings.Contains(err.Error(), "A::") {
			t.Fatalf("%v: error does not name A::: %v", order, err)
		}
	}
}

func TestSelfCheckPackageUnparseableNameIsMissing(t *testing.T) {
	s := loadSelfCheckFixture(t, SourceFile{Name: "model.sysml", Text: `package M { part def P; }`})

	verdicts, err := s.SelfCheckPackages([]string{"Acme::"})
	if verdicts != nil {
		t.Fatalf("verdicts = %+v, want nil", verdicts)
	}
	if !errors.Is(err, ErrSelfCheckPackageNotFound) {
		t.Fatalf("err = %v, want ErrSelfCheckPackageNotFound", err)
	}
	if !strings.Contains(err.Error(), "Acme::") {
		t.Fatalf("error does not name Acme::: %v", err)
	}
}

// Relationship notation is reflected as metaobjects, so the relationship-valued
// constraints of the validation library evaluate over them.
func TestSelfCheckReflectiveRelationshipConstraints(t *testing.T) {
	const sysmlSrc = `
package M {
    part def T;
    port def Pd;
    part def Base {
        part c;
    }
    part def D :> Base {
        part a;
        part b subsets a;
        part e references a;
        part d :>> c;
        port p : ~Pd;
    }
    part def S :> T;
    action def B;
    attribute def AD;
}`
	const kermlSrc = `
class ShoppingCart { feature selectedProducts : Product[*]; }
class Product { feature inCart : ShoppingCart[0..1]; }
assoc ProductSelection {
    end feature cart : ShoppingCart[1] crosses selectedProduct.inCart;
    end feature selectedProduct : Product[1] crosses cart.selectedProducts;
}
class K;
assoc A {
    end feature x : K;
    end feature y : K;
}
assoc A2 specializes A {
    end feature x2 redefines x;
    end feature y2 redefines y;
}
class KB { feature f1; }
class KD specializes KB {
    feature g redefines f1;
}
class K2 conjugates KB;
`
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{
		{Name: "rel_self_check.sysml", Text: sysmlSrc},
		{Name: "rel_self_check.kerml", Text: kermlSrc},
	})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixtures did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixtures have model errors")
	}
	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", false)
	for _, constraint := range []string{
		"validateSpecializationSpecificNotConjugated",
		"validateBehaviorSpecialization",
		"validateStructureSpecialization",
		"validateClassSpecialization",
		"validateDataTypeSpecialization",
		"validateFeatureOwnedReferenceSubsetting",
		"validateFeatureOwnedCrossSubsetting",
	} {
		if stats.applications["SysMLValidation::"+constraint] == 0 {
			t.Errorf("%s was never applied", constraint)
		}
	}
	if stats.violations != 0 {
		t.Errorf("clean fixtures produced %d violations: %+v", stats.violations, verdicts)
	}
	if stats.evaluationErrors != 0 {
		t.Errorf("relationship constraints had %d evaluation errors: %+v", stats.evaluationErrors, verdicts)
	}
}

func TestSelfCheckKerMLNotationTypesKeepBaseClassification(t *testing.T) {
	file := SourceFile{Name: "kerml_notation.sysml", Text: `package P {
	class x;
	metadata def service;
	#service def S;
	#service s;
}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}
	_, stats := s.selfCheckWithCounts("SysMLValidation", true)
	if stats.violations != 0 || stats.evaluationErrors != 0 {
		t.Errorf("self-check had %d violations and %d errors", stats.violations, stats.evaluationErrors)
	}
}

// A class specializing a datatype violates validateClassSpecialization: the
// library states the strong form, which the published OCL's precedence drops.
func TestSelfCheckClassSpecializingDataType(t *testing.T) {
	file := SourceFile{Name: "class_datatype.kerml", Text: `class C specializes D;
datatype D;
`}
	s := NewSession()
	s.SubmitFiles([]SourceFile{file})
	verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
	assertSelfCheckViolation(t, verdicts, "validateClassSpecialization", "C")
}

func TestSelfCheckRelationshipConstraintViolations(t *testing.T) {
	for _, tt := range []struct {
		name       string
		source     SourceFile
		constraint string
		element    string
	}{
		{name: "behavior specializes struct",
			source:     SourceFile{Name: "bh.kerml", Text: "behavior B specializes S;\nstruct S;\n"},
			constraint: "validateBehaviorSpecialization", element: "B"},
		{name: "struct specializes behavior",
			source:     SourceFile{Name: "st.kerml", Text: "struct S specializes B;\nbehavior B;\n"},
			constraint: "validateStructureSpecialization", element: "S"},
		{name: "datatype specializes class",
			source:     SourceFile{Name: "dc.kerml", Text: "datatype D specializes C;\nclass C;\n"},
			constraint: "validateDataTypeSpecialization", element: "D"},
		{name: "datatype specializes assoc",
			source:     SourceFile{Name: "da.kerml", Text: "datatype D specializes A;\nassoc A;\n"},
			constraint: "validateDataTypeSpecialization", element: "D"},
		{name: "feature with two references",
			source:     SourceFile{Name: "rr.sysml", Text: "package P {\n\tpart def B;\n\tpart def A :> B {\n\t\tpart a;\n\t\tpart r references a references b;\n\t\tpart b;\n\t}\n}"},
			constraint: "validateFeatureOwnedReferenceSubsetting", element: "r"},
		{name: "feature with two crosses",
			source:     SourceFile{Name: "cc.kerml", Text: "class K {\n\tfeature f;\n\tfeature g;\n}\nclass L :> K {\n\tfeature x crosses f crosses g;\n}"},
			constraint: "validateFeatureOwnedCrossSubsetting", element: "x"},
		{name: "conjugated specific",
			source:     SourceFile{Name: "cj.kerml", Text: "class B;\nclass A conjugates B;\nspecialization S subtype A specializes B;\n"},
			constraint: "validateSpecializationSpecificNotConjugated", element: "S"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			s.SubmitFiles([]SourceFile{tt.source})
			verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
			assertSelfCheckViolation(t, verdicts, tt.constraint, tt.element)
		})
	}
}

// A variation definition or usage must not specialize another variation. The
// w8d pass already rejects the model, but the constraint still reports the
// violation once the model is held for self-check.
func TestSelfCheckVariationSpecializesVariation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		source     SourceFile
		constraint string
		element    string
	}{
		{name: "definition",
			source:     SourceFile{Name: "vd.sysml", Text: "package P {\n\tvariation part def V;\n\tvariation part def W :> V;\n}"},
			constraint: "validateDefinitionVariationSpecialization", element: "W"},
		{name: "usage",
			source:     SourceFile{Name: "vu.sysml", Text: "package P {\n\tpart def T;\n\tvariation part v : T;\n\tvariation part w :> v;\n}"},
			constraint: "validateUsageVariationSpecialization", element: "w"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			s.SubmitFiles([]SourceFile{tt.source})
			verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
			assertSelfCheckViolation(t, verdicts, tt.constraint, tt.element)
		})
	}
}

// Variations that specialize no variation pass the variation specialization
// constraints.
func TestSelfCheckVariationSpecializesNonVariation(t *testing.T) {
	file := SourceFile{Name: "vpass.sysml", Text: `package P {
	part def T;
	variation part def V;
	variation part v : T;
}`}
	s := NewSession()
	result := s.SubmitFiles([]SourceFile{file})
	if errs := errorDiagnostics(result.Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture did not load cleanly: %v", errs)
	}
	if s.HasErrors() {
		t.Fatal("fixture has model errors")
	}
	verdicts, stats := s.selfCheckWithCounts("SysMLValidation", false)
	if stats.applications["SysMLValidation::validateDefinitionVariationSpecialization"] == 0 {
		t.Fatal("validateDefinitionVariationSpecialization was never applied")
	}
	if stats.applications["SysMLValidation::validateUsageVariationSpecialization"] == 0 {
		t.Fatal("validateUsageVariationSpecialization was never applied")
	}
	if stats.violations != 0 {
		t.Errorf("clean fixture produced %d violations: %+v", stats.violations, verdicts)
	}
}
