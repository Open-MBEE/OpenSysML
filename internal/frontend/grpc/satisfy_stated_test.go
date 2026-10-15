package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// statedSatisfyModel states a satisfaction in a definition, with no `by`, and
// verifies the same requirement from an objective that states nothing else.
const statedSatisfyModel = `
package RequirementPkg {
	private import ScalarValues::Real;
	private import DefinitionPkg::Definition;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}
}
package DefinitionPkg {
	private import ScalarValues::Real;
	private import RequirementPkg::*;

	part def Definition {
		attribute cost : Real;
		satisfy costReq;
	}
}
package ProductPkg {
	private import DefinitionPkg::*;
	private import RequirementPkg::*;

	part product : Definition {
		attribute :>> cost = 80.0;
	}
	part costly : Definition {
		attribute :>> cost = 120.0;
	}

	verification def CostTest {
		subject p : Definition;
		objective costObjective { verify costReq; }
		return verdict = VerificationCases::PassIf(p.cost <= 100.0);
	}
	verification costTest : CostTest { subject p = product; }
}
`

// TestVerifySatisfactionStatedByADefinitionSweepsHeldObjects checks that
// VerifySatisfaction decides a definition's `satisfy r;` once per object of the
// definition the model holds, asks for one when it holds none, and reports no
// verdict for the objective's `verify`.
func TestVerifySatisfactionStatedByADefinitionSweepsHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, statedSatisfyModel, "stated-satisfy")
	ctx := context.Background()

	resp, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("verdicts with no object = %v, want the one assertion", resp.Verdicts)
	}
	if v := resp.Verdicts[0]; v.Status != statusUndecided || !strings.Contains(v.Error, "an object of DefinitionPkg::Definition is needed") {
		t.Errorf("verdict with no object = %v, want undecided asking for an object of the definition", v)
	}

	ids := map[string]int64{}
	for _, name := range []string{"ProductPkg::product", "ProductPkg::costly"} {
		made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: name})
		if err != nil || made.Error != "" {
			t.Fatalf("Instantiate(%s): %v %q", name, err, made.GetError())
		}
		ids[name] = made.Instance.Id
	}
	resp, err = srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(resp.Verdicts) != 2 {
		t.Fatalf("verdicts = %v, want one per object of the definition", resp.Verdicts)
	}
	holds := map[int64]bool{}
	for _, v := range resp.Verdicts {
		if v.Element != "satisfy costReq" || v.Error != "" || v.RequirementId != "RequirementPkg::costReq" {
			t.Errorf("verdict = %v, want a decided satisfy costReq naming its requirement", v)
		}
		holds[v.InstanceId] = v.Holds
	}
	if !holds[ids["ProductPkg::product"]] || holds[ids["ProductPkg::costly"]] {
		t.Errorf("holds by object = %v, want product true and costly false (%v)", holds, ids)
	}
	wantVerdicts(t, "VerifySatisfaction", resp.VerificationVerdicts, "ProductPkg::costTest=pass")

	// A second object of product is displaced from its name but still held, and
	// is an object of the definition, so the sweep decides it too.
	made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "ProductPkg::product"})
	if err != nil || made.Error != "" {
		t.Fatalf("Instantiate again: %v %q", err, made.GetError())
	}
	resp, err = srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	decided := map[int64]bool{}
	for _, v := range resp.Verdicts {
		decided[v.InstanceId] = true
	}
	if len(resp.Verdicts) != 3 || !decided[ids["ProductPkg::product"]] || !decided[made.Instance.Id] || !decided[ids["ProductPkg::costly"]] {
		t.Errorf("verdicts after a second product = %v, want one per held object (%v, %d)", resp.Verdicts, ids, made.Instance.Id)
	}
}
