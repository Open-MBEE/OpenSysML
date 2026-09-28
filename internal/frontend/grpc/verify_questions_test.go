package grpc

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// requireSolver skips the test where no SMT solver is installed, unless the
// suite requires one — the convention the solve package's own tests follow.
func requireSolver(t *testing.T) {
	t.Helper()
	if _, err := solve.Discover(); err != nil {
		if os.Getenv("OPENSYSML_REQUIRE_SMT") != "" {
			t.Fatalf("no SMT solver found, but OPENSYSML_REQUIRE_SMT is set: %v", err)
		}
		t.Skipf("no SMT solver installed: %v", err)
	}
}

// symbolicModelSource is the model the solver questions are asked of: the
// design's lemma and counterexample, a quantity, an enum/boolean gate, a
// requirement and a satisfaction, a nonlinear claim and a subject to pin.
const symbolicModelSource = `package P {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	part def HG { attribute efficiency : Real; attribute power : Real; }
	part hg : HG;
	attribute d : Real;
	assert constraint lemma {
		(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
		implies (hg.power * d * hg.efficiency) <= hg.power * d
	}
	assert constraint bad { hg.power * d <= hg.power }
	assert constraint never { hg.power > 1.0 and hg.power < 0.0 }
	assert constraint sq { hg.power ** 2.0 >= 0.0 }

	part engine { attribute power : PowerValue; }
	assert constraint nonneg { engine.power >= 0 [SI::W] implies engine.power * 2.0 >= engine.power }
	assert constraint cap { engine.power <= 100 [SI::W] }

	enum def Mode { enum on; enum off; }
	part light { attribute m : Mode; attribute b : Boolean; }
	assert constraint total { light.m == Mode::on or light.m == Mode::off }
	assert constraint gate { light.b implies light.m == Mode::on }

	requirement def Bounds {
		subject thing : HG;
		attribute limit : Real;
		assume constraint { thing.power >= 0.0 }
		require constraint { thing.power <= 0.0 implies thing.power < limit }
	}
	requirement bounded : Bounds { attribute :>> limit = 100.0; }
	requirement steep : Bounds { attribute :>> limit = -1.0; }

	part def Rig {
		attribute power : Real = 1.0;
		attribute d : Real = 0.5;
		assert constraint bad { power * d <= power }
	}
}
`

// mustSymbolicModel parses the symbolic model and returns its hash.
func mustSymbolicModel(t *testing.T, srv *Service) string {
	t.Helper()
	return mustVerifyModel(t, srv, symbolicModelSource, "verify-questions")
}

// verifyQuestion asks a question of a constraint and fails the test on a fault.
func verifyQuestion(t *testing.T, srv *Service, hash, symbol, question, subject, engine string) *pb.VerifyConstraintResponse {
	t.Helper()
	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        symbol,
		Question:        question,
		SubjectSymbolId: subject,
		Engine:          engine,
	})
	if err != nil {
		t.Fatalf("VerifyConstraint(%s, %s): %v", symbol, question, err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint(%s, %s) reported %q", symbol, question, resp.Error)
	}
	return resp
}

// TestVerifyQuestionsHoldsProvesTheLemma: the design's worked example — its
// violation is unsat, proved under the evaluator's float64 arithmetic.
func TestVerifyQuestionsHoldsProvesTheLemma(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "P::lemma", "holds", "", "")
	if !resp.Verdict.Holds || resp.Verdict.Status != statusHolds {
		t.Fatalf("verdict is holds=%v status=%q, want proved holds: %q", resp.Verdict.Holds, resp.Verdict.Status, resp.Verdict.Error)
	}
	if resp.Verdict.Question != questionHolds {
		t.Errorf("question = %q, want holds", resp.Verdict.Question)
	}
	if resp.Verdict.Strength != "proved" {
		t.Errorf("strength = %q, want proved", resp.Verdict.Strength)
	}
}

// TestVerifyQuestionsHoldsWitnessesAViolation: `bad` fails for some
// assignment, and the witness is the violating one the evaluator replayed.
func TestVerifyQuestionsHoldsWitnessesAViolation(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "P::bad", "holds", "", "")
	if resp.Verdict.Holds || resp.Verdict.Status != statusViolated {
		t.Fatalf("verdict is holds=%v status=%q, want violated", resp.Verdict.Holds, resp.Verdict.Status)
	}
	if resp.Verdict.Strength != "witnessed" {
		t.Errorf("strength = %q, want witnessed", resp.Verdict.Strength)
	}
	if len(resp.Verdict.Witness) == 0 {
		t.Fatal("a violated verdict names no witness")
	}
	var power, d float64
	var sawPower, sawD bool
	for _, w := range resp.Verdict.Witness {
		switch w.Feature {
		case "P::hg.power":
			power, sawPower = w.Value.GetRealValue(), true
		case "P::d":
			d, sawD = w.Value.GetRealValue(), true
		}
		if w.Exact == "" {
			t.Errorf("witness %s names no exact value", w.Feature)
		}
	}
	if !sawPower || !sawD {
		t.Fatalf("witness misses hg.power or d: %+v", resp.Verdict.Witness)
	}
	if power*d <= power {
		t.Errorf("witness %g * %g = %g <= %g does not violate `hg.power * d <= hg.power`", power, d, power*d, power)
	}
}

// TestVerifyQuestionsSatisfiableWitnessesAndRefuses: `bad` is satisfiable, a
// witnessed assignment; `never` is not, proved.
func TestVerifyQuestionsSatisfiableWitnessesAndRefuses(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)

	resp := verifyQuestion(t, srv, hash, "P::bad", "satisfiable", "", "")
	if !resp.Verdict.Holds || resp.Verdict.Status != statusSatisfiable {
		t.Fatalf("verdict is holds=%v status=%q, want satisfiable", resp.Verdict.Holds, resp.Verdict.Status)
	}
	if len(resp.Verdict.Witness) == 0 {
		t.Error("a satisfiable verdict names no witness")
	}

	never := verifyQuestion(t, srv, hash, "P::never", "satisfiable", "", "")
	if never.Verdict.Holds || never.Verdict.Status != statusUnsatisfiable {
		t.Errorf("verdict is holds=%v status=%q, want unsatisfiable", never.Verdict.Holds, never.Verdict.Status)
	}
}

// TestVerifyQuestionsQuantities: the non-negativity implication holds over the
// quantity's free values; the cap does not, witnessed in the base units.
func TestVerifyQuestionsQuantities(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)

	if resp := verifyQuestion(t, srv, hash, "P::nonneg", "holds", "", ""); resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
		t.Errorf("nonneg: status=%q holds=%v, want proved holds: %q", resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
	}
	resp := verifyQuestion(t, srv, hash, "P::cap", "holds", "", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("cap: status=%q, want violated", resp.Verdict.Status)
	}
	var unit string
	for _, w := range resp.Verdict.Witness {
		unit = w.Unit
		if w.Value.GetRealValue() <= 100 {
			t.Errorf("witness %s = %v does not violate the cap", w.Feature, w.Value)
		}
	}
	if unit == "" {
		t.Error("the witness reports no base units")
	}
}

// TestVerifyQuestionsEnumAndBoolean: the enum covering holds; the gate is
// violated by b true with m off.
func TestVerifyQuestionsEnumAndBoolean(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)

	if resp := verifyQuestion(t, srv, hash, "P::total", "holds", "", ""); !resp.Verdict.Holds || resp.Verdict.Status != statusHolds {
		t.Errorf("total: status=%q holds=%v, want proved holds: %q", resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
	}
	resp := verifyQuestion(t, srv, hash, "P::gate", "holds", "", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("gate: status=%q, want violated", resp.Verdict.Status)
	}
	var b, m bool
	for _, w := range resp.Verdict.Witness {
		switch w.Feature {
		case "P::light.b":
			if w.Value.GetBoolValue() {
				b = true
			}
		case "P::light.m":
			if strings.HasSuffix(w.Exact, "off") {
				m = true
			}
		}
	}
	if !b || !m {
		t.Errorf("witness %+v is not b = true with m = Mode::off", resp.Verdict.Witness)
	}
}

// TestVerifyQuestionsRequirement: the requirement's claim is its assumption
// implying its requirement — bounded holds under a free power, steep is
// violated since the assumption permits a nonnegative power over its limit.
func TestVerifyQuestionsRequirement(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash: hash, SymbolId: "P::bounded", Question: "holds",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
		t.Errorf("bounded: status=%q holds=%v, want proved holds: %q", resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
	}

	steep, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash: hash, SymbolId: "P::steep", Question: "holds",
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if steep.Verdict.Status != statusViolated || len(steep.Verdict.Witness) == 0 {
		t.Errorf("steep: status=%q, want a witnessed violation: %q", steep.Verdict.Status, steep.Verdict.Error)
	}
}

// TestVerifyQuestionsSatisfaction: each assertion is asked one violation
// query per verdict.
func TestVerifyQuestionsSatisfaction(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	source := `package P {
	private import ScalarValues::*;
	part def Craft { attribute mass : Real; }
	requirement def MassLimit {
		subject vehicle : Craft;
		attribute cap : Real;
		require constraint { vehicle.mass <= cap }
	}
	requirement massLimit : MassLimit { attribute :>> cap = 2000.0; }
	part craft : Craft { attribute :>> mass = 1200.0; }
	part analysis { assert satisfy massLimit by craft; }
}
`
	hash := mustVerifyModel(t, srv, source, "verify-satisfy-questions")
	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash, Question: "holds",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want the one assertion's: %v", len(resp.Verdicts), resp.Verdicts)
	}
	if resp.Verdicts[0].Question != questionHolds || resp.Verdicts[0].Status != statusHolds {
		t.Errorf("verdict is question=%q status=%q error=%q, want holds", resp.Verdicts[0].Question, resp.Verdicts[0].Status, resp.Verdicts[0].Error)
	}
}

// TestVerifyQuestionsSubjectPins: against the object whose values the subject
// holds, `bad` holds — the pinned values witness no violation. Free, the same
// constraint is violated.
func TestVerifyQuestionsSubjectPins(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "P::Rig::bad", "holds", "P::Rig", "")
	if resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
		t.Errorf("status=%q holds=%v, want the pinned claim to hold: %q", resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
	}
	free := verifyQuestion(t, srv, hash, "P::bad", "holds", "", "")
	if free.Verdict.Status != statusViolated {
		t.Errorf("unpinned status=%q, want violated", free.Verdict.Status)
	}
}

// TestVerifyQuestionsNonlinearIsUndecided: exponentiation translates only
// under a guard a violation query cannot hoist, so the question is undecided
// naming the refusal — needing no solver.
func TestVerifyQuestionsNonlinearIsUndecided(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "P::sq", "holds", "", "")
	if resp.Verdict.Status != statusUndecided || resp.Verdict.Holds {
		t.Fatalf("verdict is status=%q holds=%v, want undecided", resp.Verdict.Status, resp.Verdict.Holds)
	}
	if resp.Verdict.Error == "" {
		t.Error("an undecided verdict names no reason")
	}
	if resp.Verdict.FailureReason != pb.FailureReason_FAILURE_REASON_UNDECIDED {
		t.Errorf("failure_reason = %v, want UNDECIDED", resp.Verdict.FailureReason)
	}
	if len(resp.Verdict.Witness) != 0 {
		t.Error("an undecided verdict names a witness")
	}
}

// TestVerifyQuestionsBogusIsInvalid: a question spelling the API does not know
// is the caller's error, not an undecided verdict.
func TestVerifyQuestionsBogusIsInvalid(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	_, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash, SymbolId: "P::lemma", Question: "probably",
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want INVALID_ARGUMENT", connect.CodeOf(err))
	}
}

// TestVerifyQuestionsCapabilityWithheld: the solver questions need their
// capability; evaluate does not.
func TestVerifyQuestionsCapabilityWithheld(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityVerificationQuestions)
	hash := mustSymbolicModel(t, srv)
	_, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash, SymbolId: "P::lemma", Question: "holds",
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("code = %v, want the capability's error", connect.CodeOf(err))
	}
	resp := verifyQuestion(t, srv, hash, "P::lemma", "", "", "")
	if resp.Verdict.Question != questionEvaluate {
		t.Errorf("question = %q, want evaluate", resp.Verdict.Question)
	}
}

// TestVerifyQuestionsEngineRefusalIsUndecided: a question put to an engine
// answering none of it is undecided naming the refusal.
func TestVerifyQuestionsEngineRefusalIsUndecided(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "P::lemma", "holds", "", "run")
	if resp.Verdict.Status != statusUndecided || resp.Verdict.Error == "" {
		t.Fatalf("verdict is status=%q error=%q, want undecided naming the refusal", resp.Verdict.Status, resp.Verdict.Error)
	}
	if !strings.Contains(resp.Verdict.Error, "holds") {
		t.Errorf("error = %q, want it to name the question refused", resp.Verdict.Error)
	}
}

// TestVerifyQuestionsOmittedIsEvaluate: an unset question is the evaluation it
// always was, the new fields naming what was answered.
func TestVerifyQuestionsOmittedIsEvaluate(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyModelSource, "verify-evaluate-stamps")
	resp := verifyQuestion(t, srv, hash, "Demo::Vehicle::massPositive", "", "Demo::sedan", "")
	if !resp.Verdict.Holds {
		t.Fatal("massPositive on sedan does not hold")
	}
	if resp.Verdict.Question != questionEvaluate || resp.Verdict.Status != statusHolds {
		t.Errorf("stamps are question=%q status=%q, want evaluate holds", resp.Verdict.Question, resp.Verdict.Status)
	}
	failing := verifyQuestion(t, srv, hash, "Demo::Vehicle::massLight", "", "Demo::sedan", "")
	if failing.Verdict.Status != statusViolated {
		t.Errorf("failing status = %q, want violated", failing.Verdict.Status)
	}
	if len(resp.Verdict.Witness) != 0 {
		t.Error("an evaluation names no witness")
	}
}

// TestVerifyQuestionsCapabilityReported verifies the capability is advertised.
func TestVerifyQuestionsCapabilityReported(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityVerificationQuestions) {
		t.Errorf("capabilities = %v, want %q", info.Capabilities, CapabilityVerificationQuestions)
	}
}
