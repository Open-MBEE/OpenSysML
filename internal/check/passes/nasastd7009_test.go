package passes

import (
	"strings"
	"testing"
)

// nasastd7009Model wraps body in a package importing the NASASTD7009 library.
func nasastd7009Model(body string) string {
	return "package M {\n\tprivate import NASASTD7009::*;\n\tprivate import NASASTD7009::MSRequirements::*;" + body + "\n}"
}

var nasastd7009Codes = []string{
	CodeNASASTD7009AssessmentMissingFactor,
	CodeNASASTD7009ReportMissingContent,
	CodeNASASTD7009ReportMissingWarning,
	CodeNASASTD7009WarningMissingImpact,
	CodeNASASTD7009CriticalityRating,
	CodeNASASTD7009EntryIncomplete,
	CodeNASASTD7009MatrixMissingReq,
}

const nasastd7009FullCapability = `
			#capabilityFactor item pedigree { :>> factor = CapabilityFactor::msDataPedigree; :>> level = FactorLevel::level3; }
			#capabilityFactor item verified { :>> factor = CapabilityFactor::msVerification; :>> level = FactorLevel::level3; }
			#capabilityFactor item validated { :>> factor = CapabilityFactor::msValidation; :>> level = FactorLevel::level2; }
			#capabilityFactor item reviewed { :>> factor = CapabilityFactor::msDevelopmentTechnicalReview; :>> level = FactorLevel::level1; }
			#capabilityFactor item managed { :>> factor = CapabilityFactor::msDevelopmentProcessProductManagement; :>> level = FactorLevel::level2; }`

const nasastd7009FullResults = `
			#resultsFactor item use { :>> factor = ResultsFactor::msUseAssessment; :>> level = FactorLevel::level3; }
			#resultsFactor item inputs { :>> factor = ResultsFactor::msInputPedigree; :>> level = FactorLevel::level3; }
			#resultsFactor item uncertainty { :>> factor = ResultsFactor::msUncertaintyCharacterization; :>> level = FactorLevel::level2; }
			#resultsFactor item robustness { :>> factor = ResultsFactor::msResultsRobustness; :>> level = FactorLevel::level2; }
			#resultsFactor item reviewed { :>> factor = ResultsFactor::msUseAnalysisTechnicalReview; :>> level = FactorLevel::level1; }
			#resultsFactor item managed { :>> factor = ResultsFactor::msUseProcessProductManagement; :>> level = FactorLevel::level2; }`

// An assessment records a level for each of its factors; a factor with no
// assessment, or one stating no level, is missing, and a complete record is not reported.
func TestNASASTD7009AssessmentMissingFactor(t *testing.T) {
	src := nasastd7009Model(`
		#capabilityAssessment item complete {` + nasastd7009FullCapability + `
		}
		#capabilityAssessment item partial {
			#capabilityFactor item pedigree { :>> factor = CapabilityFactor::msDataPedigree; :>> level = FactorLevel::level3; }
			#capabilityFactor item verified { :>> factor = CapabilityFactor::msVerification; }
		}
		#resultsAssessment item results {
			part group {
				#resultsFactor item use { :>> factor = ResultsFactor::msUseAssessment; :>> level = FactorLevel::level4; }
			}
		}
		#resultsAssessment item all {` + nasastd7009FullResults + `
		}`)
	w8dWantLines(t, src, CodeNASASTD7009AssessmentMissingFactor, 11, 15)
	diags := w8dDiags(t, src)
	for _, d := range diags {
		if d.Code != CodeNASASTD7009AssessmentMissingFactor {
			continue
		}
		if strings.Contains(d.Message, "capability") && !strings.Contains(d.Message, "4 of its 5 factors: M&S Verification, M&S Validation") {
			t.Errorf("capability finding does not name the four factors in the Standard's order: %s", d.Message)
		}
		if strings.Contains(d.Message, "results assessment") && !strings.Contains(d.Message, "5 of its 6 factors: M&S Input Pedigree") {
			t.Errorf("results finding does not name the five factors: %s", d.Message)
		}
	}
}

// A report includes both assessments, the uncertainty estimate with its process, the qualifications
// and the risk acceptance; no estimate available needs no process, and inherited content counts.
func TestNASASTD7009ReportMissingContent(t *testing.T) {
	src := nasastd7009Model(`
		part def Report :> MSResultsReport {
			:>> uncertaintyEstimate = UncertaintyEstimateKind::notAvailable;
			:>> qualifications = "The analysts hold the thermal analyst certification.";
			:>> riskAcceptance = "Accepted by the project manager.";
			#capabilityAssessment item capability {` + nasastd7009FullCapability + `
			}
			#resultsAssessment item results {` + nasastd7009FullResults + `
			}
		}
		part complete : Report;
		#resultsReport part bare;
		#resultsReport part noProcess : Report {
			:>> uncertaintyEstimate = UncertaintyEstimateKind::quantitative;
		}
		#resultsReport part withProcess : Report {
			:>> uncertaintyEstimate = UncertaintyEstimateKind::quantitative;
			:>> uncertaintyProcess = "Monte Carlo over the input distributions.";
		}`)
	w8dWantLines(t, src, CodeNASASTD7009ReportMissingContent, 25, 26)
	for _, d := range w8dDiags(t, src) {
		if d.Code != CodeNASASTD7009ReportMissingContent {
			continue
		}
		bare := strings.Contains(d.Message, "[M&S 50]")
		for _, id := range []string{"[M&S 35]", "[M&S 33]", "[M&S 37]", "[M&S 39]"} {
			if bare != strings.Contains(d.Message, id) {
				t.Errorf("finding names %s inconsistently: %s", id, d.Message)
			}
		}
		if bare == strings.Contains(d.Message, "[M&S 34]") {
			t.Errorf("only the report with an estimate and no process lacks [M&S 34]: %s", d.Message)
		}
	}
}

// A report holding a factor assessed below its threshold carries a warning of unachieved
// acceptance criteria; a factor at its threshold, or with none, asks for no warning.
func TestNASASTD7009ReportMissingWarning(t *testing.T) {
	report := func(name, factor, extra string) string {
		return `
		#resultsReport part ` + name + ` {
			:>> uncertaintyEstimate = UncertaintyEstimateKind::notAvailable;
			:>> qualifications = "q";
			:>> riskAcceptance = "r";
			#capabilityAssessment item capability {` + nasastd7009FullCapability + `
			}
			#resultsAssessment item results {` + nasastd7009FullResults + `
				` + factor + `
			}` + extra + `
		}`
	}
	below := `#resultsFactor item again { :>> factor = ResultsFactor::msResultsRobustness; :>> level = FactorLevel::level1; :>> threshold = FactorLevel::level2; }`
	met := `#resultsFactor item again { :>> factor = ResultsFactor::msResultsRobustness; :>> level = FactorLevel::level2; :>> threshold = FactorLevel::level2; }`
	warning := `
			#resultsWarning item gap { :>> kind = WarningOccurrence::unachievedAcceptanceCriteria; :>> impact = "Margins are less certain."; }`
	other := `
			#resultsWarning item limits { :>> kind = WarningOccurrence::violatedLimits; :>> impact = "None expected."; }`
	src := nasastd7009Model(report("unwarned", below, "") + report("warned", below, warning) +
		report("otherWarning", below, other) + report("met", met, ""))
	lines := w8dLines(t, src, CodeNASASTD7009ReportMissingWarning)
	if len(lines) != 2 {
		t.Fatalf("got lines %v, want the unwarned report and the one warning of something else", lines)
	}
	for _, d := range w8dDiags(t, src) {
		if d.Code == CodeNASASTD7009ReportMissingWarning && !strings.Contains(d.Message, "M&S Results Robustness (level 1, threshold 2)") {
			t.Errorf("finding does not name the factor and its gap: %s", d.Message)
		}
	}
}

// A warning states the impact of its occurrence, wherever it is declared.
func TestNASASTD7009WarningMissingImpact(t *testing.T) {
	src := nasastd7009Model(`
		#resultsWarning item stated { :>> kind = WarningOccurrence::violatedLimits; :>> impact = "Low."; }
		#resultsWarning item bare { :>> kind = WarningOccurrence::violatedLimits; }
		item typed : ResultsWarning { :>> kind = WarningOccurrence::waivers; :>> impact = ""; }`)
	w8dWantLines(t, src, CodeNASASTD7009WarningMissingImpact, 5, 6)
}

// A rating that differs from Figure 1 is reported unless the assessment records its method;
// every cell of the figure is read off the library.
func TestNASASTD7009CriticalityRating(t *testing.T) {
	src := nasastd7009Model(`
		part agrees { @CriticalityAssessment { consequence = DecisionConsequence::significant; influence = MSInfluence::controlling; rating = CriticalityRating::red; } }
		part differs { @CriticalityAssessment { consequence = DecisionConsequence::significant; influence = MSInfluence::controlling; rating = CriticalityRating::yellow; } }
		part ownMethod { @CriticalityAssessment { consequence = DecisionConsequence::significant; influence = MSInfluence::controlling; rating = CriticalityRating::yellow; method = "Program matrix"; } }
		part unrated { @CriticalityAssessment { consequence = DecisionConsequence::significant; influence = MSInfluence::controlling; } }`)
	w8dWantLines(t, src, CodeNASASTD7009CriticalityRating, 5)

	figure := map[string]string{
		"controlling": "GYRRR",
		"significant": "GYYRR",
		"moderate":    "GYYYR",
		"minor":       "GGGYY",
		"negligible":  "GGGGY",
	}
	ratings := map[byte]string{'G': "green", 'Y': "yellow", 'R': "red"}
	consequences := []string{"negligible", "minor", "moderate", "significant", "catastrophic"}
	var body strings.Builder
	want := 0
	for influence, row := range figure {
		for i, consequence := range consequences {
			for _, rating := range []string{"green", "yellow", "red"} {
				if rating != ratings[row[i]] {
					want++
				}
				body.WriteString("\n\t\tpart " + influence + consequence + rating + " { @CriticalityAssessment { consequence = DecisionConsequence::" +
					consequence + "; influence = MSInfluence::" + influence + "; rating = CriticalityRating::" + rating + "; } }")
			}
		}
	}
	got := w8dLines(t, nasastd7009Model(body.String()), CodeNASASTD7009CriticalityRating)
	if len(got) != want {
		t.Fatalf("got %d findings over the 75 ratings of the 25 cells, want the %d that differ from Figure 1", len(got), want)
	}
}

// A matrix entry says why a requirement is not applicable and where the record of an
// applicable requirement is kept; a recommendation needs no record location.
func TestNASASTD7009RequirementEntryIncomplete(t *testing.T) {
	src := nasastd7009Model(`
		part entries {
			requirement a : MS40 { @Applicability { applicable = Applicable::yes; recordLocation = "docs/intended-use.md"; } }
			requirement b : MS6 { @Applicability { applicable = Applicable::yes; } }
			requirement c : MS17 { @Applicability { applicable = Applicable::no; comments = "Validated by the vendor."; } }
			requirement d : MS18 { @Applicability { applicable = Applicable::no; } }
			requirement e : MSRecommendations::R4_1_2a { @Applicability { applicable = Applicable::yes; } }
			requirement f : MSRecommendations::R4_1_2b { @Applicability { applicable = Applicable::no; } }
		}`)
	w8dWantLines(t, src, CodeNASASTD7009EntryIncomplete, 6, 8, 10)
}

// A Requirements Identification Matrix is reported once, naming the requirements with no entry.
func TestNASASTD7009MatrixMissingRequirement(t *testing.T) {
	src := nasastd7009Model(`
		part entries {
			requirement a : MS40 { @Applicability { applicable = Applicable::yes; recordLocation = "docs/intended-use.md"; } }
			requirement b : MS6 { @Applicability { applicable = Applicable::no; comments = "Not a critical decision."; } }
			requirement c : MS41;
		}
		part def Matrix :> RequirementsIdentificationMatrix;
		part matrix : Matrix;`)
	w8dWantLines(t, src, CodeNASASTD7009MatrixMissingReq, 10)
	for _, d := range w8dDiags(t, src) {
		if d.Code != CodeNASASTD7009MatrixMissingReq {
			continue
		}
		if !strings.Contains(d.Message, "41 of the 43 requirements of NASA-STD-7009B: [M&S 41], [M&S 42]") {
			t.Errorf("finding does not count the requirements or name them in the Standard's order: %s", d.Message)
		}
		if strings.Contains(d.Message, "[M&S 40]") || strings.Contains(d.Message, "[M&S 6]") {
			t.Errorf("finding names a requirement that has an entry: %s", d.Message)
		}
	}
}

// A model that never uses the library, and a workspace package that merely reuses its
// name, get no finding.
func TestNASASTD7009SilentWithoutTheLibrary(t *testing.T) {
	for _, src := range []string{
		`package M { part def Report; part report : Report; item warning; }`,
		`package NASASTD7009 { part def MSResultsReport; item def ResultsWarning; }
		package M { private import NASASTD7009::*; part report : MSResultsReport; item warning : ResultsWarning; }`,
	} {
		for _, d := range w8dDiags(t, src) {
			for _, code := range nasastd7009Codes {
				if d.Code == code {
					t.Errorf("unexpected %s: %s", code, d.Message)
				}
			}
		}
	}
}
