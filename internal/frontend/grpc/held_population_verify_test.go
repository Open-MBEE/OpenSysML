package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// heldPopulationModel owns a verification case in a definition two objects of
// which a holder repeats; the case's verdict calls the requirement on `this`.
const heldPopulationModel = `package test {
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
			return verdict = VerificationCases::PassIf(costReq(this));
		}
	}
	part def Fleet { part cheap : Definition [2] { attribute :>> cost = 80.0; } }
	part fleet : Fleet;
}`

// TestVerifyRequirementWithoutASubjectSweepsTheHeldPopulation verifies that
// VerifyRequirement naming no subject answers over the objects Instantiate holds,
// as the REPL's %requirement does after %instantiate: the requirement's own verdict
// is undecided, its subject being unbound, and the definition-owned case runs once
// on each held object of the definition — the same verdicts VerifySatisfaction reports.
func TestVerifyRequirementWithoutASubjectSweepsTheHeldPopulation(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, heldPopulationModel, "held-population-requirement")
	ctx := context.Background()
	made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "test::fleet"})
	if err != nil || made.Error != "" {
		t.Fatalf("Instantiate(test::fleet): %v %q", err, made.GetError())
	}

	satisfied, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	wantVerdicts(t, "VerifySatisfaction", satisfied.VerificationVerdicts, "test::Definition::costTest=pass", "test::Definition::costTest=pass")

	required, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "test::costReq"})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if required.Error != "" {
		t.Fatalf("VerifyRequirement reported %q", required.Error)
	}
	if v := required.Verdict; v.Status != statusUndecided || !strings.Contains(v.Error, "product subject is unbound") {
		t.Errorf("requirement verdict = %v, want undecided with the subject unbound, as the REPL answers", v)
	}
	wantVerdicts(t, "VerifyRequirement", required.VerificationVerdicts, "test::Definition::costTest=pass", "test::Definition::costTest=pass")
	wantVerdictObjects(t, "VerifyRequirement", required.VerificationVerdicts, required.Instances, "test::Fleet::cheap", "test::Fleet::cheap")
	// The verdicts are about the held objects, the ones VerifySatisfaction decided.
	heldObjects := map[int64]bool{}
	for _, v := range satisfied.VerificationVerdicts {
		heldObjects[v.InstanceId] = true
	}
	for _, v := range required.VerificationVerdicts {
		if !heldObjects[v.InstanceId] {
			t.Errorf("VerifyRequirement verdict on object #%d, which the model does not hold (held %v)", v.InstanceId, heldObjects)
		}
	}
}
