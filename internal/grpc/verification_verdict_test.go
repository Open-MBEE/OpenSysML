package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// verificationVerdictModel states a requirement verified by two verification
// cases, one whose body passes and one whose body fails, so every surface that
// reports a satisfaction verdict has a body verdict to report beside it.
const verificationVerdictModel = `package Demo {
	private import ScalarValues::*;

	part def Widget {
		attribute m : Integer default = 0;
	}

	part good : Widget;
	part bad : Widget {
		attribute :>> m = 1;
	}

	requirement def Zeroed {
		subject w : Widget;
		require constraint { w.m == 0 }
	}

	requirement zeroed : Zeroed {
		subject w = good;
	}

	verification def Check {
		subject w : Widget;
		objective { verify zeroed; }
		VerificationCases::PassIf(w.m == 0)
	}

	verification checkGood : Check {
		subject w = good;
	}

	verification checkBad : Check {
		subject w = bad;
	}

	verification def Plan {
		subject w : Widget;
		objective { verify zeroed; }
		verification sub : Check;
	}

	verification plan : Plan {
		subject w = bad;
	}

	part checks {
		assert satisfy zeroed by good;
	}
}
`

// bodyVerdicts renders the reported body verdicts as "case=kind", marking a
// subcase, so a test states the whole answer in one comparison.
func bodyVerdicts(verdicts []*pb.VerificationVerdict) []string {
	out := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		line := verdict.CaseId + "=" + verdict.Kind
		if verdict.Subcase {
			line += "(subcase)"
		}
		out = append(out, line)
	}
	return out
}

func wantVerdicts(t *testing.T, surface string, got []*pb.VerificationVerdict, want ...string) {
	t.Helper()
	if lines := strings.Join(bodyVerdicts(got), ","); lines != strings.Join(want, ",") {
		t.Errorf("%s verification verdicts = %v, want %v", surface, bodyVerdicts(got), want)
	}
}

// TestVerifyRequirementReportsTheBodyVerdicts verifies the requirement's own
// verdict is unchanged and the verdict of each verifying case's body is
// reported beside it.
func TestVerifyRequirementReportsTheBodyVerdicts(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verificationVerdictModel, "verification-verdicts")

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::zeroed",
		SubjectSymbolId: "Demo::good",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyRequirement reported %q", resp.Error)
	}
	if !resp.Verdict.Holds {
		t.Errorf("zeroed by good: holds = false, want true (%s)", resp.Verdict.Condition)
	}
	wantVerdicts(t, "VerifyRequirement", resp.VerificationVerdicts,
		"Demo::checkGood=pass", "Demo::checkBad=fail",
		"Demo::plan=inconclusive", "Demo::Plan::sub=fail(subcase)")
	for _, verdict := range resp.VerificationVerdicts {
		if verdict.Kind == "inconclusive" && verdict.Detail == "" {
			t.Errorf("%s decided nothing and says nothing about why", verdict.CaseId)
		}
	}
}

// TestVerifySatisfactionReportsTheBodyVerdicts verifies the same beside the
// verdict of each satisfaction assertion.
func TestVerifySatisfactionReportsTheBodyVerdicts(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verificationVerdictModel, "verification-verdicts-satisfy")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	for _, verdict := range resp.Verdicts {
		if !verdict.Holds {
			t.Errorf("%q: holds = false, want true (%s)", verdict.Element, verdict.Condition)
		}
	}
	wantVerdicts(t, "VerifySatisfaction", resp.VerificationVerdicts,
		"Demo::checkGood=pass", "Demo::checkBad=fail",
		"Demo::plan=inconclusive", "Demo::Plan::sub=fail(subcase)")
}

// twoRequirementModel asserts two requirements satisfied, each verified by a
// case of its own, so one response reports the cases of both.
const twoRequirementModel = `package Demo {
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

// TestVerifySatisfactionAssociatesBodyVerdictsWithTheirRequirement verifies a
// response covering two requirements names the requirement each body verdict
// was reported for, and that the verdict of an assertion names the same one, so
// a client keeps them apart rather than reading one requirement's cases as
// another's.
func TestVerifySatisfactionAssociatesBodyVerdictsWithTheirRequirement(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, twoRequirementModel, "verification-verdicts-two")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	byRequirement := map[string][]string{}
	for _, verdict := range resp.VerificationVerdicts {
		byRequirement[verdict.RequirementId] = append(
			byRequirement[verdict.RequirementId], verdict.CaseId+"="+verdict.Kind)
	}
	for requirement, want := range map[string]string{
		"Demo::zeroed": "Demo::checkZero=pass", "Demo::bounded": "Demo::checkBound=fail",
	} {
		if got := strings.Join(byRequirement[requirement], ","); got != want {
			t.Errorf("verdicts of %s = %q, want %q", requirement, got, want)
		}
	}
	if len(resp.Verdicts) == 0 {
		t.Fatal("VerifySatisfaction reported no assertion verdict")
	}
	for _, verdict := range resp.Verdicts {
		if _, ok := byRequirement[verdict.RequirementId]; !ok {
			t.Errorf("%q names requirement %q, which no body verdict was reported for",
				verdict.Element, verdict.RequirementId)
		}
	}
}

// declaredSatisfactionModel states its requirement in the satisfaction itself
// (`satisfy requirement r by p`), so the requirement a case verifies is that
// usage rather than one written elsewhere.
const declaredSatisfactionModel = `package Demo {
	private import ScalarValues::*;

	part def Widget {
		attribute m : Integer default = 0;
	}

	part good : Widget;

	assert satisfy requirement zeroed by good {
		subject w : Widget;
		require constraint { w.m == 0 }
	}

	verification def Check {
		subject w : Widget;
		objective { verify zeroed; }
		VerificationCases::PassIf(w.m == 0)
	}

	verification checkGood : Check { subject w = good; }
}
`

// TestVerifySatisfactionOfADeclaredRequirementReportsItsBodyVerdicts verifies a
// satisfaction declaring its own requirement is associated with it too, so the
// cases verifying that requirement are reported and named for it.
func TestVerifySatisfactionOfADeclaredRequirementReportsItsBodyVerdicts(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, declaredSatisfactionModel, "verification-verdicts-declared")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	wantVerdicts(t, "VerifySatisfaction", resp.VerificationVerdicts, "Demo::checkGood=pass")
	if len(resp.Verdicts) == 0 {
		t.Fatal("VerifySatisfaction reported no assertion verdict")
	}
	for _, verdict := range resp.Verdicts {
		if verdict.RequirementId != "Demo::zeroed" {
			t.Errorf("%q names requirement %q, want Demo::zeroed", verdict.Element, verdict.RequirementId)
		}
	}
	for _, verdict := range resp.VerificationVerdicts {
		if verdict.RequirementId != "Demo::zeroed" {
			t.Errorf("%s names requirement %q", verdict.CaseId, verdict.RequirementId)
		}
	}
}

// TestRunAnalysisRunsAVerificationCase verifies a verification case is accepted
// by the run RPC rather than refused, reporting its body verdict and the verdict
// of each subcase it performs.
func TestRunAnalysisRunsAVerificationCase(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verificationVerdictModel, "verification-verdicts-run")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash: hash,
		SymbolId:  "Demo::checkBad",
	})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis of a verification case reported %q", resp.Error)
	}
	wantVerdicts(t, "RunAnalysis", resp.VerificationVerdicts, "Demo::checkBad=fail")

	plan := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash: hash,
		SymbolId:  "Demo::plan",
	})
	wantVerdicts(t, "RunAnalysis of a plan", plan.VerificationVerdicts,
		"Demo::plan=inconclusive", "Demo::Plan::sub=fail(subcase)")
}

// TestRunAnalysisOfAVerificationCaseWithoutASubjectIsAnErrorVerdict verifies a
// body that cannot run is the case's error verdict carrying the message, not a
// failure of the call.
func TestRunAnalysisOfAVerificationCaseWithoutASubjectIsAnErrorVerdict(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verificationVerdictModel, "verification-verdicts-error")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Check",
	})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q, want an error verdict", resp.Error)
	}
	if len(resp.VerificationVerdicts) != 1 || resp.VerificationVerdicts[0].Kind != "error" {
		t.Fatalf("verdicts = %v, want one error verdict", bodyVerdicts(resp.VerificationVerdicts))
	}
	if !strings.Contains(resp.VerificationVerdicts[0].Detail, "subject") {
		t.Errorf("error verdict detail = %q, want the unbound subject", resp.VerificationVerdicts[0].Detail)
	}
}

// TestCapabilitiesAdvertiseVerificationVerdicts verifies a client can ask
// whether the service reports body verdicts before reading the field.
func TestCapabilitiesAdvertiseVerificationVerdicts(t *testing.T) {
	srv := mustNewService(t, 10)
	resp, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	for _, capability := range resp.Capabilities {
		if capability == CapabilityVerificationVerdicts {
			return
		}
	}
	t.Errorf("capabilities %v do not advertise %q", resp.Capabilities, CapabilityVerificationVerdicts)
}
