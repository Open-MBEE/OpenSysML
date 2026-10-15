package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// definitionOwnedCaseModel declares a verification case inside a part
// definition, whose body reads the owning object's cost.
const definitionOwnedCaseModel = `package Cost {
	private import ScalarValues::Real;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement { :>> maximumCost = 100.0; }

	part def Definition {
		attribute cost : Real;
		satisfy costReq;
		verification costTest {
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(cost <= 100.0);
		}
	}

	part cheap : Definition { attribute :>> cost = 80.0; }
	part dear : Definition { attribute :>> cost = 120.0; }

	part checks {
		assert satisfy costReq by cheap;
		assert satisfy costReq by dear;
	}
}`

// TestRunAnalysisRunsADefinitionOwnedCaseOnTheSubjectGiven verifies the object
// given to a case a definition owns is the object it runs as a feature of.
func TestRunAnalysisRunsADefinitionOwnedCaseOnTheSubjectGiven(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, definitionOwnedCaseModel, "definition-owned-run")

	for _, tc := range []struct{ subject, want string }{
		{"Cost::cheap", "Cost::Definition::costTest=pass"},
		{"Cost::dear", "Cost::Definition::costTest=fail"},
	} {
		resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{
			ModelHash:       hash,
			SymbolId:        "Cost::Definition::costTest",
			SubjectSymbolId: tc.subject,
		})
		if resp.Error != "" {
			t.Fatalf("RunAnalysis on %s reported %q", tc.subject, resp.Error)
		}
		wantVerdicts(t, "RunAnalysis on "+tc.subject, resp.VerificationVerdicts, tc.want)
		// The case declares no subject, so the object it ran on is reported as
		// the one its verdict is about, not as the run's subject.
		wantVerdictObjects(t, "RunAnalysis on "+tc.subject, resp.VerificationVerdicts, resp.Instances, tc.subject)
	}
}

// TestValidateInstanceRunsADefinitionOwnedCaseOnTheObject verifies the cases
// verifying a requirement a validated object satisfies run on that object.
func TestValidateInstanceRunsADefinitionOwnedCaseOnTheObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, definitionOwnedCaseModel, "definition-owned-validate")

	for _, tc := range []struct{ object, want string }{
		{"Cost::cheap", "Cost::Definition::costTest=pass"},
		{"Cost::dear", "Cost::Definition::costTest=fail"},
	} {
		resp, err := srv.ValidateInstance(context.Background(), &pb.ValidateInstanceRequest{
			ModelHash: hash,
			SymbolId:  tc.object,
		})
		if err != nil {
			t.Fatalf("ValidateInstance(%s): %v", tc.object, err)
		}
		if resp.Error != "" {
			t.Fatalf("ValidateInstance(%s) reported %q", tc.object, resp.Error)
		}
		wantVerdicts(t, "ValidateInstance of "+tc.object, resp.VerificationVerdicts, tc.want)
	}

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Cost::costReq",
		SubjectSymbolId: "Cost::dear",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	wantVerdicts(t, "VerifyRequirement on dear", resp.VerificationVerdicts, "Cost::Definition::costTest=fail")

	// Two assertions of one requirement about two objects: the case answers
	// about each object, not once for the requirement.
	satisfied, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  "Cost::checks",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if satisfied.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", satisfied.Error)
	}
	wantVerdicts(t, "VerifySatisfaction of checks", satisfied.VerificationVerdicts,
		"Cost::Definition::costTest=pass", "Cost::Definition::costTest=fail")
	wantVerdictObjects(t, "VerifySatisfaction of checks", satisfied.VerificationVerdicts, satisfied.Instances,
		"Cost::cheap", "Cost::dear")
}

// wantVerdictObjects verifies each verdict names the object its case ran on, by
// id and type, and that the id resolves to an instance of the response.
func wantVerdictObjects(t *testing.T, surface string, got []*pb.VerificationVerdict, instances []*pb.Instance, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s reported %d verdicts, want %d", surface, len(got), len(want))
	}
	byID := make(map[int64]*pb.Instance, len(instances))
	for _, inst := range instances {
		byID[inst.Id] = inst
	}
	for i, verdict := range got {
		inst, ok := byID[verdict.InstanceId]
		if !ok {
			t.Errorf("%s verdict %d names object #%d, which the response does not carry", surface, i, verdict.InstanceId)
			continue
		}
		if inst.TypeSymbolId != want[i] {
			t.Errorf("%s verdict %d is about %s (#%d), want %s", surface, i, inst.TypeSymbolId, inst.Id, want[i])
		}
		if verdict.InstanceTypeId != inst.TypeSymbolId {
			t.Errorf("%s verdict %d names type %q, want %q", surface, i, verdict.InstanceTypeId, inst.TypeSymbolId)
		}
	}
}
