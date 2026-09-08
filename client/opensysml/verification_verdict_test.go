package opensysml_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// verdictSource asserts two requirements satisfied, each verified by a case of
// its own, so one response reports the cases of both.
const verdictSource = `package Demo {
	private import ScalarValues::*;

	part def Widget {
		attribute m : Integer default = 0;
	}

	part good : Widget;

	requirement def Zeroed {
		subject w : Widget;
		require constraint { w.m == 0 }
	}

	requirement zeroed : Zeroed { subject w = good; }
	requirement bounded : Zeroed { subject w = good; }

	verification def ZeroCheck {
		subject w : Widget;
		objective { verify zeroed; }
		VerificationCases::PassIf(w.m == 0)
	}

	verification def BoundCheck {
		subject w : Widget;
		objective { verify bounded; }
		VerificationCases::PassIf(w.m == 1)
	}

	verification checkZero : ZeroCheck { subject w = good; }
	verification checkBound : BoundCheck { subject w = good; }

	part checks {
		assert satisfy zeroed by good;
		assert satisfy bounded by good;
	}
}
`

// bodyVerdicts renders reported body verdicts as "case=kind", marking a subcase.
func bodyVerdicts(verdicts []opensysml.VerificationVerdict) []string {
	out := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		line := verdict.CaseID + "=" + string(verdict.Kind)
		if verdict.Subcase {
			line += "(subcase)"
		}
		out = append(out, line)
	}
	return out
}

func TestVerifyRequirementReportsTheVerifyingBodiesVerdicts(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verdictSource)

	verification, err := client.VerifyRequirement(context.Background(), model, "Demo::zeroed",
		opensysml.Against("Demo::good"))
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if !verification.Verdict.Holds {
		t.Errorf("zeroed by good: holds = false, want true (%s)", verification.Verdict.Condition)
	}
	if got := strings.Join(bodyVerdicts(verification.Verifications), ","); got != "Demo::checkZero=pass" {
		t.Errorf("verifications = %v, want the verifying case's passing body verdict", got)
	}
	if got := verification.Verifications[0].RequirementID; got != "Demo::zeroed" {
		t.Errorf("requirement of the body verdict = %q, want Demo::zeroed", got)
	}
}

// TestVerifySatisfactionGivesEachVerdictItsOwnRequirementsBodies verifies a
// response covering two requirements is read per requirement, rather than every
// verdict exposing the whole response's cases.
func TestVerifySatisfactionGivesEachVerdictItsOwnRequirementsBodies(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verdictSource)

	satisfaction, err := client.VerifySatisfaction(context.Background(), model, "")
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if got := bodyVerdicts(satisfaction.Verifications); !slices.Contains(got, "Demo::checkZero=pass") ||
		!slices.Contains(got, "Demo::checkBound=fail") {
		t.Fatalf("verifications = %v, want the body verdict of each requirement's case", got)
	}
	for _, verdict := range satisfaction.Verdicts {
		for _, body := range verdict.Verifications {
			if body.RequirementID != verdict.RequirementID {
				t.Errorf("%q carries %s, reported for %q, not for %q",
					verdict.Element, body.CaseID, body.RequirementID, verdict.RequirementID)
			}
		}
	}
	byRequirement := map[string][]string{}
	for _, verdict := range satisfaction.Verdicts {
		byRequirement[verdict.RequirementID] = bodyVerdicts(verdict.Verifications)
	}
	for requirement, want := range map[string]string{
		"Demo::zeroed": "Demo::checkZero=pass", "Demo::bounded": "Demo::checkBound=fail",
	} {
		if got := strings.Join(byRequirement[requirement], ","); got != want {
			t.Errorf("bodies of %s = %q, want %q", requirement, got, want)
		}
	}
}

func TestRunAnalysisReportsTheVerificationCaseBodyVerdict(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verdictSource)

	analysis, err := client.RunAnalysis(context.Background(), model, "Demo::checkBound")
	if err != nil {
		t.Fatalf("RunAnalysis: %v", err)
	}
	if got := strings.Join(bodyVerdicts(analysis.Verifications), ","); got != "Demo::checkBound=fail" {
		t.Errorf("verifications = %v, want the case's own failing body verdict", got)
	}
	if got := analysis.Verifications[0].RequirementID; got != "" {
		t.Errorf("requirement of a case run for itself = %q, want none", got)
	}
}

// TestAServiceReportingNoBodyVerdictsAnswersNone verifies a constraint, which
// no verification case is about, reports none rather than another's.
func TestAServiceReportingNoBodyVerdictsAnswersNone(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)

	verification, err := client.VerifyConstraint(context.Background(), model, "Demo::Vehicle::massLight",
		opensysml.Against("Demo::sedan"))
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if verification.Verifications != nil {
		t.Errorf("verifications = %v, want none", bodyVerdicts(verification.Verifications))
	}
}

func TestTheClientNamesTheVerificationVerdictCapability(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !slices.Contains(info.Capabilities, opensysml.CapabilityVerificationVerdicts) {
		t.Errorf("capabilities = %v, want it to contain %q",
			info.Capabilities, opensysml.CapabilityVerificationVerdicts)
	}
}
