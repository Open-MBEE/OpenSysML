package grpc

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// verifyModelSource states a constraint, a requirement and satisfaction
// assertions that hold and that fail, so one model exercises every verdict the
// verification RPCs report.
const verifyModelSource = `package Demo {
	part def Vehicle {
		attribute mass default = 1500.0;

		constraint massPositive {
			mass > 0.0
		}

		constraint massLight {
			mass < 100.0
		}

		requirement lightEnough {
			require constraint { mass < 2000.0 }
		}

		requirement tiny {
			require constraint { mass < 10.0 }
		}
	}

	requirement def MassLimit {
		subject vehicle : Vehicle;
		attribute maxMass;
		require constraint {
			vehicle.mass <= maxMass
		}
	}

	requirement massLimit : MassLimit {
		attribute :>> maxMass = 2000.0;
	}

	requirement massTiny : MassLimit {
		attribute :>> maxMass = 10.0;
	}

	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}

	part analysis {
		assert satisfy massLimit by sedan;
		assert satisfy massTiny by sedan;
	}

	calc add {
		in x;
		in y;
		x + y
	}
}
`

// mustVerifyModel parses the verification model and returns its hash.
func mustVerifyModel(t *testing.T, srv *Service, source, hash string) string {
	t.Helper()
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: source},
		ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range resp.Diagnostics {
		if strings.EqualFold(diag.Severity, "error") {
			t.Fatalf("model has errors: %v", resp.Diagnostics)
		}
	}
	return resp.ModelHash
}

// TestVerificationCapabilityReported verifies a client can require verification
// before asking for it.
func TestVerificationCapabilityReported(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityVerification) {
		t.Errorf("capabilities = %v, want it to contain %q", info.Capabilities, CapabilityVerification)
	}
}

// TestVerifyConstraintHoldsAndFails verifies a constraint's verdict is reported
// for the subject named, and that a condition evaluating to false is reported as
// a verdict rather than as an error.
func TestVerifyConstraintHoldsAndFails(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-constraint")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::massPositive",
		SubjectSymbolId: "Demo::sedan",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint reported %q", resp.Error)
	}
	if !resp.Verdict.Holds {
		t.Errorf("massPositive on sedan: holds = false, want true (%s)", resp.Verdict.Condition)
	}
	if resp.Verdict.Kind != "constraint" {
		t.Errorf("kind = %q, want %q", resp.Verdict.Kind, "constraint")
	}
	if resp.Verdict.ElementId != "Demo::Vehicle::massPositive" {
		t.Errorf("element_id = %q, want %q", resp.Verdict.ElementId, "Demo::Vehicle::massPositive")
	}
	if resp.Verdict.InstanceId == 0 {
		t.Error("verdict names no instance, so its values cannot be read")
	}
	if len(resp.Instances) == 0 {
		t.Error("no instance graph returned for the subject")
	}

	failing, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::massLight",
		SubjectSymbolId: "Demo::sedan",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if failing.Error != "" {
		t.Fatalf("a false condition is a verdict, not an error: %q", failing.Error)
	}
	if failing.Verdict.Holds {
		t.Error("massLight on sedan: holds = true, want false")
	}
	if failing.Verdict.Condition == "" {
		t.Error("a failing verdict names no condition")
	}
}

// TestVerifyRequirementHoldsAndFails verifies the same for requirements, whose
// verdict is over their required constraints.
func TestVerifyRequirementHoldsAndFails(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-requirement")

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::lightEnough",
		SubjectSymbolId: "Demo::sedan",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyRequirement reported %q", resp.Error)
	}
	if !resp.Verdict.Holds {
		t.Errorf("lightEnough on sedan: holds = false, want true (%s)", resp.Verdict.Condition)
	}
	if resp.Verdict.Kind != "requirement" {
		t.Errorf("kind = %q, want %q", resp.Verdict.Kind, "requirement")
	}

	failing, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::tiny",
		SubjectSymbolId: "Demo::sedan",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if failing.Verdict.Holds {
		t.Error("tiny on sedan: holds = true, want false")
	}
	if failing.Error != "" {
		t.Errorf("a false requirement is a verdict, not an error: %q", failing.Error)
	}
}

// TestVerifyConstraintUnknownSymbol verifies an unanswerable request is reported
// in the response rather than as a transport failure, as the other runtime RPCs
// report theirs.
func TestVerifyConstraintUnknownSymbol(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-unknown-symbol")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash,
		SymbolId:  "Demo::NoSuchConstraint",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint should not fail the call: %v", err)
	}
	if !strings.Contains(resp.Error, "NoSuchConstraint") {
		t.Errorf("error = %q, want it to name the missing symbol", resp.Error)
	}
}

// TestVerifyConstraintUnknownSubject verifies a subject that cannot be
// instantiated is reported as such, not as a failing verdict.
func TestVerifyConstraintUnknownSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-unknown-subject")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::massPositive",
		SubjectSymbolId: "Demo::nosuchpart",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint should not fail the call: %v", err)
	}
	if !strings.Contains(resp.Error, "nosuchpart") {
		t.Errorf("error = %q, want it to name the missing subject", resp.Error)
	}
}

// TestVerifyUncachedModelIsNotFound verifies an evicted model is NOT FOUND, so a
// client can tell it apart from a model that answers.
func TestVerifyUncachedModelIsNotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	for name, call := range map[string]func() error{
		"VerifyConstraint": func() error {
			_, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
				ModelHash: "nosuchmodel", SymbolId: "Demo::Vehicle::massPositive",
			})
			return err
		},
		"VerifyRequirement": func() error {
			_, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
				ModelHash: "nosuchmodel", SymbolId: "Demo::Vehicle::lightEnough",
			})
			return err
		},
		"VerifySatisfaction": func() error {
			_, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
				ModelHash: "nosuchmodel",
			})
			return err
		},
		"EvaluateCalc": func() error {
			_, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
				ModelHash: "nosuchmodel", SymbolId: "Demo::add",
			})
			return err
		},
	} {
		err := call()
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s on an uncached model: code = %v, want NotFound", name, connect.CodeOf(err))
		}
	}
}

// TestVerifySatisfactionEvaluatesEveryAssertion verifies %satisfy's answer is
// scriptable: one verdict per assertion the model states, in declaration order.
func TestVerifySatisfactionEvaluatesEveryAssertion(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-satisfy")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	if len(resp.Verdicts) != 2 {
		t.Fatalf("got %d verdicts, want 2: %v", len(resp.Verdicts), resp.Verdicts)
	}
	if !resp.Verdicts[0].Holds {
		t.Errorf("satisfy massLimit by sedan: holds = false, want true (%s)", resp.Verdicts[0].Condition)
	}
	if resp.Verdicts[1].Holds {
		t.Error("satisfy massTiny by sedan: holds = true, want false")
	}
	for i, verdict := range resp.Verdicts {
		if verdict.Kind != "satisfy" {
			t.Errorf("verdict %d: kind = %q, want %q", i, verdict.Kind, "satisfy")
		}
		if verdict.Element == "" {
			t.Errorf("verdict %d names no assertion", i)
		}
		if verdict.InstanceId == 0 {
			t.Errorf("verdict %d is about no object, so its values cannot be read", i)
		}
	}
	seen := map[int64]bool{}
	for _, inst := range resp.Instances {
		if seen[inst.Id] {
			t.Errorf("instance %d reported twice", inst.Id)
		}
		seen[inst.Id] = true
	}
}

// TestVerifySatisfactionNarrowedToASymbol verifies a symbol limits evaluation to
// the assertions that element states.
func TestVerifySatisfactionNarrowedToASymbol(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package Demo {
	part def Vehicle {
		attribute mass default = 1500.0;
	}
	requirement def MassLimit {
		subject vehicle : Vehicle;
		attribute maxMass;
		require constraint { vehicle.mass <= maxMass }
	}
	requirement massLimit : MassLimit {
		attribute :>> maxMass = 2000.0;
	}
	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}
	part group {
		assert satisfy massLimit by sedan;
	}
	part other {
		assert satisfy massLimit by sedan;
	}
}
`
	hash := mustVerifyModel(t, srv, source, "verify-satisfy-narrowed")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  "Demo::group",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want the one Demo::group states: %v", len(resp.Verdicts), resp.Verdicts)
	}
	if !resp.Verdicts[0].Holds {
		t.Errorf("holds = false, want true (%s)", resp.Verdicts[0].Condition)
	}
}

// TestVerifySatisfactionNarrowedToANamedAssertion verifies a `satisfy
// requirement <name> : <def> by <part>` usage is itself one assertion, so naming
// it evaluates that assertion rather than the assertions of its scope.
func TestVerifySatisfactionNarrowedToANamedAssertion(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package Demo {
	part def Vehicle {
		attribute mass default = 1500.0;
	}
	requirement def MassLimit {
		subject vehicle : Vehicle;
		attribute maxMass;
		require constraint { vehicle.mass <= maxMass }
	}
	requirement massLimit : MassLimit {
		attribute :>> maxMass = 2000.0;
	}
	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}
	part group {
		satisfy requirement limitMet : MassLimit by sedan;
		assert satisfy massLimit by sedan;
	}
}
`
	hash := mustVerifyModel(t, srv, source, "verify-satisfy-named")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  "Demo::group::limitMet",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want the named assertion alone: %v", len(resp.Verdicts), resp.Verdicts)
	}
	if resp.Verdicts[0].Kind != "satisfy" {
		t.Errorf("kind = %q, want %q", resp.Verdicts[0].Kind, "satisfy")
	}
}

// TestVerifySatisfactionOfAnElementStatingNone verifies the answer for an element
// that asserts nothing says so rather than reporting a passing model.
func TestVerifySatisfactionOfAnElementStatingNone(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-satisfy-none")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Vehicle",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(resp.Verdicts) != 0 {
		t.Errorf("got %d verdicts, want none", len(resp.Verdicts))
	}
}

// TestEvaluateCalcInvokesWithArguments verifies %calc's invocation form.
func TestEvaluateCalcInvokesWithArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-calc-args")

	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::add",
		Arguments: []*pb.Value{
			{Kind: &pb.Value_RealValue{RealValue: 2.5}},
			{Kind: &pb.Value_RealValue{RealValue: 4.0}},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc reported %q", resp.Error)
	}
	if got := resp.Result.GetRealValue(); got != 6.5 {
		t.Errorf("result = %v, want 6.5 (result: %v)", got, resp.Result)
	}
}

// TestEvaluateCalcUsageReportsItsOutputs verifies a calc usage named with no
// arguments is evaluated from its own members, as SysML 7.17 specifies and as
// %calc does at the prompt.
func TestEvaluateCalcUsageReportsItsOutputs(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package Demo {
	calc def Two {
		in n;
		out a = n + 1;
		out b = n * 2;
	}
	calc c : Two {
		in n = 5;
	}
}
`
	hash := mustVerifyModel(t, srv, source, "verify-calc-usage")

	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::c",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc reported %q", resp.Error)
	}
	if len(resp.Outputs) != 2 {
		t.Fatalf("got %d outputs, want a and b: %v", len(resp.Outputs), resp.Outputs)
	}
	want := map[string]int64{"a": 6, "b": 10}
	for _, out := range resp.Outputs {
		if out.Value.GetIntValue() != want[out.Name] {
			t.Errorf("output %s = %v, want %d", out.Name, out.Value, want[out.Name])
		}
	}
}

// A calc usage evaluated from its members is one run under the registry like any
// other evaluation, so a caller already gone fails the call unperformed.
func TestEvaluateCalcUsageCanceledCallerFailsTheCall(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `package Demo {
	calc def Two { in n; out a = n + 1; }
	calc c : Two { in n = 5; }
}
`, "verify-calc-usage-canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: "Demo::c"})
	if !errors.Is(err, context.Canceled) || resp != nil {
		t.Fatalf("EvaluateCalc for a gone caller: %v, %v; want context.Canceled and no response", err, resp)
	}
}

// TestEvaluateCalcUnknownSymbol verifies an unknown name is reported in the
// response rather than as a transport failure.
func TestEvaluateCalcUnknownSymbol(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-calc-unknown")

	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::NoSuchCalc",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc should not fail the call: %v", err)
	}
	if !strings.Contains(resp.Error, "NoSuchCalc") {
		t.Errorf("error = %q, want it to name the missing symbol", resp.Error)
	}
}

// TestVerifyWrongKindIsClassified verifies naming an element of another kind is
// reported as a wrong request, distinguishably from an undecided verdict.
func TestVerifyWrongKindIsClassified(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-wrong-kind")

	constraint, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Vehicle",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if constraint.Verdict.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
		t.Errorf("constraint: failure_reason = %v, want WRONG_KIND (%q)",
			constraint.Verdict.FailureReason, constraint.Verdict.Error)
	}

	requirement, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Vehicle",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if requirement.Verdict.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
		t.Errorf("requirement: failure_reason = %v, want WRONG_KIND (%q)",
			requirement.Verdict.FailureReason, requirement.Verdict.Error)
	}

	calc, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Vehicle",
		Arguments: []*pb.Value{{Kind: &pb.Value_IntValue{IntValue: 1}}},
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if calc.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
		t.Errorf("calc: failure_reason = %v, want WRONG_KIND (%q)", calc.FailureReason, calc.Error)
	}

	// An element with a body may state an assertion, so only one that can hold
	// no members at all is the wrong kind of thing to narrow satisfaction to.
	if got := failureReason(runtime.ErrNotASatisfaction); got != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
		t.Errorf("satisfy: failureReason(ErrNotASatisfaction) = %v, want WRONG_KIND", got)
	}
}

// TestVerifySatisfactionOfAnAliasIsWrongKind verifies narrowing satisfaction to an
// element that can state none is reported as a wrong request.
func TestVerifySatisfactionOfAnAliasIsWrongKind(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `package Demo {
	part def Vehicle;
	alias V for Vehicle;
}
`, "verify-satisfy-alias")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  "Demo::V",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
		t.Errorf("failure_reason = %v, want WRONG_KIND (%q)", resp.FailureReason, resp.Error)
	}
}

// TestVerdictFalseCarriesNoFailureReason verifies a model answering false stays a
// verdict, so a client reading the reason cannot mistake it for a bad request.
func TestVerdictFalseCarriesNoFailureReason(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-false-reason")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Vehicle::massLight",
		SubjectSymbolId: "Demo::sedan",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Verdict.Holds {
		t.Fatal("massLight on sedan: holds = true, want false")
	}
	if resp.Verdict.FailureReason != pb.FailureReason_FAILURE_REASON_UNSPECIFIED {
		t.Errorf("failure_reason = %v, want UNSPECIFIED", resp.Verdict.FailureReason)
	}
}

// TestVerifyConstraintConcurrentRequestsShareTheModel verifies concurrent
// requests over one model answer alike, each on a worker of its own, including
// ones that name an unknown symbol, whose resolution errors are that request's.
func TestVerifyConstraintConcurrentRequestsShareTheModel(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-concurrent")

	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			symbol, subject := "Demo::Vehicle::massPositive", "Demo::sedan"
			if i%4 == 3 {
				symbol = "Demo::NoSuchConstraint"
			}
			resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
				ModelHash: hash, SymbolId: symbol, SubjectSymbolId: subject,
			})
			switch {
			case err != nil:
				errs <- err.Error()
			case symbol == "Demo::NoSuchConstraint" && !strings.Contains(resp.Error, "NoSuchConstraint"):
				errs <- "unknown symbol not reported: " + resp.Error
			case symbol != "Demo::NoSuchConstraint" && (resp.Error != "" || !resp.Verdict.Holds):
				errs <- "massPositive did not hold: " + resp.Error
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	cached, _ := srv.cache.Get(hash)
	model, _ := cached.Semantics()
	if resolver := model.Resolver(); len(resolver.Diagnostics) != 0 || resolver.MemoSize() != 0 {
		t.Errorf("a worker built after the requests carries %d diagnostics and %d resolutions of theirs", len(resolver.Diagnostics), resolver.MemoSize())
	}
}
