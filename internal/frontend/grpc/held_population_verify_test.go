package grpc

import (
	"context"
	"fmt"
	"maps"
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

// TestVerifyRequirementWithoutASubjectSweepsTheHeldPopulation verifies that a subjectless
// VerifyRequirement runs the definition-owned case once per held object, as %requirement does.
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

// TestVerifyConstraintWithoutASubjectIsAboutTheHeldCarrier verifies a subjectless constraint
// check is about the one held carrier, ambiguous over two, on defaults with none, as %constraint.
func TestVerifyConstraintWithoutASubjectIsAboutTheHeldCarrier(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `package Tanks {
	private import ScalarValues::Real;
	part def Tank {
		attribute volume : Real default := 5.0;
		constraint positive { volume > 0.0 }
	}
	part full : Tank { attribute :>> volume = 8.0; }
	part drained : Tank { attribute :>> volume = -1.0; }
}`, "held-carrier-constraint")
	ctx := context.Background()
	verify := func(t *testing.T) *pb.Verdict {
		t.Helper()
		resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Tanks::Tank::positive"})
		if err != nil {
			t.Fatalf("VerifyConstraint: %v", err)
		}
		if resp.Error != "" {
			t.Fatalf("VerifyConstraint reported %q", resp.Error)
		}
		return resp.Verdict
	}
	if v := verify(t); v.Status != statusHolds || v.InstanceId != 0 {
		t.Errorf("verdict with no object held = %v, want holds on the declared default and no object", v)
	}

	drained, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Tanks::drained"})
	if err != nil || drained.Error != "" {
		t.Fatalf("Instantiate(drained): %v %q", err, drained.GetError())
	}
	if v := verify(t); v.Status != statusViolated || v.InstanceId != drained.Instance.Id {
		t.Errorf("verdict with drained held = %v, want violated on object #%d", v, drained.Instance.Id)
	}

	if full, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Tanks::full"}); err != nil || full.Error != "" {
		t.Fatalf("Instantiate(full): %v %q", err, full.GetError())
	}
	if v := verify(t); v.Status != statusUndecided || v.FailureReason != pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT {
		t.Errorf("verdict with two carriers held = %v, want undecided with an ambiguous subject", v)
	}
	// A named subject keeps its own object, whatever the model holds.
	resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Tanks::Tank::positive", SubjectSymbolId: "Tanks::full"})
	if err != nil {
		t.Fatalf("VerifyConstraint on full: %v", err)
	}
	if v := resp.Verdict; v.Status != statusHolds || v.InstanceTypeId != "Tanks::full" {
		t.Errorf("verdict on full = %v, want holds on an object of Tanks::full", v)
	}
}

// TestRunAnalysisWithoutASubjectRunsOnTheHeldObject verifies a subjectless definition-owned
// case runs on the one held object, is ambiguous over two, and asks for one with none.
func TestRunAnalysisWithoutASubjectRunsOnTheHeldObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, definitionOwnedCaseModel, "held-definition-owned-analysis")
	ctx := context.Background()
	const kase = "Cost::Definition::costTest"

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: kase})
	wantVerdicts(t, "RunAnalysis with no object", resp.VerificationVerdicts, kase+"=inconclusive")

	if made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Cost::dear"}); err != nil || made.Error != "" {
		t.Fatalf("Instantiate(dear): %v %q", err, made.GetError())
	}
	resp = runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: kase})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis with dear held reported %q", resp.Error)
	}
	wantVerdicts(t, "RunAnalysis with dear held", resp.VerificationVerdicts, kase+"=fail")
	wantVerdictObjects(t, "RunAnalysis with dear held", resp.VerificationVerdicts, resp.Instances, "Cost::dear")

	if made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Cost::cheap"}); err != nil || made.Error != "" {
		t.Fatalf("Instantiate(cheap): %v %q", err, made.GetError())
	}
	resp = runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: kase})
	wantVerdicts(t, "RunAnalysis with two objects held", resp.VerificationVerdicts, kase+"=error")
	if detail := resp.VerificationVerdicts[0].Detail; !strings.Contains(detail, "ambiguous subject") || !strings.Contains(detail, "cheap") || !strings.Contains(detail, "dear") {
		t.Errorf("RunAnalysis with two objects held = %q, want the ambiguity naming both carriers", detail)
	}
	// Naming the object keeps today's answer about it.
	resp = runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: kase, SubjectSymbolId: "Cost::cheap"})
	wantVerdicts(t, "RunAnalysis on cheap", resp.VerificationVerdicts, kase+"=pass")
}

// displacedObjectModel owns a verification case and a constraint in a definition
// one usage is declared of, so instantiating the usage twice leaves two held objects
// of the definition: the one the name denotes and the one it displaced.
const displacedObjectModel = `package test {
	private import ScalarValues::Real;
	requirement def Tightness { subject b : Bolt; require constraint { b.torque < 10.0 } }
	requirement tight : Tightness;
	part def Bolt {
		attribute torque : Real default = 99.0;
		constraint tightEnough { torque < 10.0 }
		verification torqueTest {
			objective { verify tight; }
			return verdict = VerificationCases::PassIf(tight(this));
		}
		satisfy tight;
	}
	part b : Bolt;
}`

// TestVerifyRequirementWithoutASubjectSweepsADisplacedObject verifies that a subjectless
// VerifyRequirement runs the definition-owned case on both objects a name instantiated twice
// holds, as VerifySatisfaction does, while a subjectless VerifyConstraint and RunAnalysis over
// the two are an ambiguous subject naming both rather than silently about the latest.
func TestVerifyRequirementWithoutASubjectSweepsADisplacedObject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, displacedObjectModel, "held-displaced-object")
	ctx := context.Background()
	var ids []int64
	for range 2 {
		made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "test::b"})
		if err != nil || made.Error != "" {
			t.Fatalf("Instantiate(test::b): %v %q", err, made.GetError())
		}
		ids = append(ids, made.Instance.Id)
	}
	if ids[0] == ids[1] {
		t.Fatalf("a second Instantiate reused object #%d", ids[0])
	}
	objectsOf := func(verdicts []*pb.VerificationVerdict) map[int64]bool {
		out := map[int64]bool{}
		for _, v := range verdicts {
			out[v.InstanceId] = true
		}
		return out
	}
	both := map[int64]bool{ids[0]: true, ids[1]: true}

	required, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "test::tight"})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if required.Error != "" {
		t.Fatalf("VerifyRequirement reported %q", required.Error)
	}
	wantVerdicts(t, "VerifyRequirement", required.VerificationVerdicts, "test::Bolt::torqueTest=fail", "test::Bolt::torqueTest=fail")
	if got := objectsOf(required.VerificationVerdicts); !maps.Equal(got, both) {
		t.Errorf("VerifyRequirement verdicts are about objects %v, want both of %v", got, both)
	}
	satisfied, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if got := objectsOf(satisfied.VerificationVerdicts); !maps.Equal(got, both) {
		t.Errorf("VerifySatisfaction verdicts are about objects %v, want both of %v", got, both)
	}

	constraint, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "test::Bolt::tightEnough"})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if v := constraint.Verdict; v.Status != statusUndecided || v.FailureReason != pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT {
		t.Errorf("VerifyConstraint over two objects of one name = %v, want undecided with an ambiguous subject", v)
	}
	for _, want := range []string{fmt.Sprintf("Bolt #%d (displaced from b)", ids[0]), fmt.Sprintf("Bolt #%d (b)", ids[1])} {
		if !strings.Contains(constraint.Verdict.Error, want) {
			t.Errorf("VerifyConstraint error %q does not name %q", constraint.Verdict.Error, want)
		}
	}
	analysis := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "test::Bolt::torqueTest"})
	wantVerdicts(t, "RunAnalysis over two objects of one name", analysis.VerificationVerdicts, "test::Bolt::torqueTest=error")
	if detail := analysis.VerificationVerdicts[0].Detail; !strings.Contains(detail, "ambiguous subject") || !strings.Contains(detail, fmt.Sprintf("Bolt #%d (displaced from b)", ids[0])) {
		t.Errorf("RunAnalysis over two objects of one name = %q, want the ambiguity naming the displaced object", detail)
	}
	// A named subject keeps a worker of its own, whatever the model holds.
	named, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "test::Bolt::tightEnough", SubjectSymbolId: "test::b"})
	if err != nil {
		t.Fatalf("VerifyConstraint on test::b: %v", err)
	}
	if v := named.Verdict; v.Status != statusViolated || v.InstanceTypeId != "test::b" {
		t.Errorf("VerifyConstraint on test::b = %v, want violated on an object of test::b", v)
	}
}
