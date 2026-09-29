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

	part def Engine {
		attribute power : Real;
		ref part other : Engine;
		assert constraint c { power <= other.power }
	}
	part eng : Engine { attribute :>> power = 5.0; }
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

// TestVerifyQuestionsPinsNoSubObjectFeature: a subject's redefinition never
// pins a feature a longer chain names through a sub-object — `vehicle.sub.power`
// is sub's power, not the subject's, so `deepLimit` is violated with
// `vehicle.sub.power` in the witness.
func TestVerifyQuestionsPinsNoSubObjectFeature(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	source := `package P {
	private import ScalarValues::*;
	part def Thing { attribute power : Real; part sub : Thing; }
	requirement def DeepLimit {
		subject vehicle : Thing;
		require constraint { vehicle.sub.power <= 4.0 }
	}
	requirement deepLimit : DeepLimit;
	part craft : Thing { attribute :>> power = 5.0; }
	part analysis { assert satisfy deepLimit by craft; }
}
`
	hash := mustVerifyModel(t, srv, source, "verify-satisfy-deep")
	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash, Question: "holds",
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want the one assertion's: %v", len(resp.Verdicts), resp.Verdicts)
	}
	v := resp.Verdicts[0]
	if v.Status != statusViolated {
		t.Fatalf("verdict is status=%q error=%q, want violated — a false proof pins sub.power", v.Status, v.Error)
	}
	var saw bool
	for _, w := range v.Witness {
		if strings.HasSuffix(w.Feature, "vehicle.sub.power") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("witness %+v names no vehicle.sub.power", v.Witness)
	}
}

// TestVerifyQuestionsPinsNoChainVarOnAnotherObject: the subject's redefinition
// pins the def-scope feature, never a chain var naming the same feature on
// another object. `other` binds no object, so reading `other.power` fails the
// way evaluating c fails — the answer is undecided, never a false holds,
// whichever order the query's variables iterate in.
func TestVerifyQuestionsPinsNoChainVarOnAnotherObject(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustSymbolicModel(t, srv)
	for range 20 {
		resp := verifyQuestion(t, srv, hash, "P::Engine::c", "holds", "P::eng", "")
		if resp.Verdict.Status == statusHolds {
			t.Fatalf("holds — a false proof pins other.power")
		}
		if resp.Verdict.Status != statusUndecided || !strings.Contains(resp.Verdict.Error, "other") {
			t.Fatalf("status=%q error=%q, want undecided naming other, which binds no object", resp.Verdict.Status, resp.Verdict.Error)
		}
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

// TestVerifyQuestionsOverflowIsNoVerdict: an assignment whose evaluation
// overflows yields no verdict — the evaluator refuses the non-finite result, so
// `evaluate` is undecided rather than violated — and a `holds` question still
// proves the lemma, since an evaluation error is no counterexample.
func TestVerifyQuestionsOverflowIsNoVerdict(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package P {
	private import ScalarValues::*;
	part hg { attribute efficiency : Real = 0.0; attribute power : Real = 1.0e200; }
	attribute d : Real = 1.0e200;
	assert constraint lemma {
		(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
		implies (hg.power * d * hg.efficiency) <= hg.power * d
	}
}
`
	hash := mustVerifyModel(t, srv, source, "verify-overflow")
	for _, question := range []string{"evaluate", ""} {
		resp := verifyQuestion(t, srv, hash, "P::lemma", question, "", "")
		v := resp.Verdict
		if v.Status != statusUndecided || v.Holds {
			t.Errorf("evaluate(%q) is status=%q holds=%v, want undecided — an overflow is no violation", question, v.Status, v.Holds)
		}
		if !strings.Contains(v.Error, "not a finite Real") {
			t.Errorf("evaluate(%q) error %q does not name the non-finite result", question, v.Error)
		}
		if v.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
			t.Errorf("evaluate(%q) reason %v, want EVALUATION", question, v.FailureReason)
		}
	}

	requireSolver(t)
	held := verifyQuestion(t, srv, hash, "P::lemma", "holds", "", "")
	if held.Verdict.Status != statusHolds || !held.Verdict.Holds {
		t.Errorf("holds is status=%q holds=%v error=%q, want proved — an evaluation error is no counterexample", held.Verdict.Status, held.Verdict.Holds, held.Verdict.Error)
	}
}

// nestedCarrierModelSource redefines a nested feature on two cars and adds a
// two-wheel van, so the symbolic path's carrier resolution is exercised the
// way evaluation resolves it.
const nestedCarrierModelSource = `package Demo {
	private import ScalarValues::*;
	part def Wheel {
		attribute pressure : Real default = 30.0;
		constraint inflated {
			pressure > 20.0
		}
	}
	part def Car {
		part wheel : Wheel;
	}
	part def Van {
		part front : Wheel;
		part back : Wheel;
	}
	part flat : Car {
		part :>> wheel {
			attribute :>> pressure = 5.0;
		}
	}
	part pumped : Car {
		part :>> wheel {
			attribute :>> pressure = 25.0;
		}
	}
	part van : Van;
}
`

// TestVerifyQuestionsAsksTheObjectThatCarries: a holds question pins the values
// of the object the element is checked on — flat's wheel at pressure 5.0
// violates inflated, pumped's at 25.0 proves it, and a van carrying two wheels
// is ambiguous, reporting the same evaluation error evaluate does.
func TestVerifyQuestionsAsksTheObjectThatCarries(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, nestedCarrierModelSource, "verify-nested-carrier")

	resp := verifyQuestion(t, srv, hash, "Demo::Wheel::inflated", "holds", "Demo::flat", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("flat: status=%q error=%q, want violated — flat.wheel.pressure = 5.0 fails > 20.0", resp.Verdict.Status, resp.Verdict.Error)
	}
	resp = verifyQuestion(t, srv, hash, "Demo::Wheel::inflated", "holds", "Demo::pumped", "")
	if resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
		t.Fatalf("pumped: status=%q holds=%v, want proved holds — pumped.wheel.pressure = 25.0", resp.Verdict.Status, resp.Verdict.Holds)
	}
	amb := verifyQuestion(t, srv, hash, "Demo::Wheel::inflated", "holds", "Demo::van", "")
	eval := verifyQuestion(t, srv, hash, "Demo::Wheel::inflated", "evaluate", "Demo::van", "")
	if eval.Verdict.Status != statusUndecided {
		t.Fatalf("evaluate on van: status=%q, want undecided on the ambiguous carrier", eval.Verdict.Status)
	}
	if amb.Verdict.Status != statusUndecided {
		t.Fatalf("holds on van: status=%q, want undecided on the ambiguous carrier", amb.Verdict.Status)
	}
	if amb.Verdict.Error != eval.Verdict.Error {
		t.Fatalf("holds error %q differs from the evaluate error %q", amb.Verdict.Error, eval.Verdict.Error)
	}
}

// chainValuesModelSource fixes feature values at any depth the evaluator reads
// them: a usage default, a def default, and a three-step chain.
const chainValuesModelSource = `package P {
	private import ScalarValues::*;
	part hg { attribute power : Real = 5.0; }
	part def HG { attribute power : Real = 5.0; }
	part hg2 : HG;
	part def Inner { attribute c : Real = 7.0; }
	part def Mid { part b : Inner; }
	part a : Mid;
	assert constraint positive { hg.power > 0.0 }
	assert constraint negative { hg.power < 0.0 }
	assert constraint defPositive { hg2.power > 0.0 }
	assert constraint deep { a.b.c > 0.0 }
}
`

// TestVerifyQuestionsPinsTheValuesChainsRead: a value the model fixes at any
// depth is pinned rather than left free — hg.power = 5.0 proves positive,
// violates negative (which hg.power, being pinned, never witnesses), and
// satisfies satisfiable; the def-default and the deep chain pin the same way.
func TestVerifyQuestionsPinsTheValuesChainsRead(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, chainValuesModelSource, "verify-chain-values")

	resp := verifyQuestion(t, srv, hash, "P::positive", "holds", "", "")
	if resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
		t.Fatalf("positive: status=%q holds=%v, want proved holds: %q", resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
	}
	resp = verifyQuestion(t, srv, hash, "P::negative", "holds", "", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("negative: status=%q, want violated — hg.power = 5.0 fails < 0.0: %q", resp.Verdict.Status, resp.Verdict.Error)
	}
	for _, w := range resp.Verdict.Witness {
		if strings.HasSuffix(w.Feature, "hg.power") {
			t.Fatalf("hg.power in witness %+v, want it pinned", w)
		}
	}
	resp = verifyQuestion(t, srv, hash, "P::positive", "satisfiable", "", "")
	if resp.Verdict.Status != statusSatisfiable {
		t.Fatalf("positive satisfiable: status=%q: %q", resp.Verdict.Status, resp.Verdict.Error)
	}
	for _, element := range []string{"P::defPositive", "P::deep"} {
		resp = verifyQuestion(t, srv, hash, element, "holds", "", "")
		if resp.Verdict.Status != statusHolds || !resp.Verdict.Holds {
			t.Fatalf("%s: status=%q holds=%v, want proved holds: %q", element, resp.Verdict.Status, resp.Verdict.Holds, resp.Verdict.Error)
		}
	}
}

// failedDefaultsModelSource has failed defaults both relevant and irrelevant to
// a query, and a satisfaction whose subject reads a failed mass value.
const failedDefaultsModelSource = `package P {
	private import ScalarValues::*;
	attribute level : Real = 1.0 / 0.0;
	attribute free : Real;
	assert constraint nonneg { level >= 0.0 }
	assert constraint freec { free >= 0.0 }
	part def Tank {
		attribute lvl : Real = 1.0 / 0.0;
		attribute spare : Real;
		assert constraint c { lvl >= 0.0 }
		assert constraint s { spare >= 0.0 }
	}
	part tank : Tank;
	part def Craft { attribute mass : Real; }
	requirement def MassLimit {
		subject vehicle : Craft;
		require constraint { vehicle.mass >= 0.0 }
	}
	requirement massLimit : MassLimit;
	part craft : Craft { attribute :>> mass = 1.0 / 0.0; }
	part analysis { assert satisfy massLimit by craft; }
}
`

func TestVerifyQuestionsAFailedDefaultIsUndecided(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, failedDefaultsModelSource, "verify-failed-defaults")
	constraints := []struct {
		symbol, subject, feature string
	}{
		{"P::nonneg", "", "level"},
		{"P::Tank::c", "P::Tank", "lvl"},
		{"P::Tank::c", "P::tank", "lvl"},
	}
	for _, question := range []string{questionHolds, questionSatisfiable} {
		for _, tc := range constraints {
			resp := verifyQuestion(t, srv, hash, tc.symbol, question, tc.subject, "")
			v := resp.Verdict
			if v.Status != statusUndecided || v.Holds {
				t.Errorf("%s on %s with %s: status=%q holds=%v, want undecided: %q",
					tc.symbol, tc.subject, question, v.Status, v.Holds, v.Error)
			}
			if !strings.Contains(v.Error, "division by zero") || !strings.Contains(v.Error, tc.feature) {
				t.Errorf("%s on %s with %s: error=%q, want division by zero and %s",
					tc.symbol, tc.subject, question, v.Error, tc.feature)
			}
			if len(v.Witness) != 0 {
				t.Errorf("%s on %s with %s: undecided verdict has witness %+v",
					tc.symbol, tc.subject, question, v.Witness)
			}
		}

		resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
			ModelHash: hash, Question: question,
		})
		if err != nil {
			t.Fatalf("VerifySatisfaction(%s): %v", question, err)
		}
		var found bool
		for _, v := range resp.Verdicts {
			if !strings.Contains(v.Element, "satisfy massLimit by craft") {
				continue
			}
			found = true
			if v.Status != statusUndecided || v.Holds {
				t.Errorf("satisfaction with %s: status=%q holds=%v, want undecided: %q",
					question, v.Status, v.Holds, v.Error)
			}
			if !strings.Contains(v.Error, "division by zero") || !strings.Contains(v.Error, "mass") {
				t.Errorf("satisfaction with %s: error=%q, want division by zero and mass", question, v.Error)
			}
			if len(v.Witness) != 0 {
				t.Errorf("satisfaction with %s: undecided verdict has witness %+v", question, v.Witness)
			}
		}
		if !found {
			t.Errorf("VerifySatisfaction(%s) returned no massLimit assertion: %v", question, resp.Verdicts)
		}
	}
}

func TestVerifyQuestionsLeavesAMissingValueFree(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, failedDefaultsModelSource, "verify-missing-values-free")

	resp := verifyQuestion(t, srv, hash, "P::freec", questionHolds, "", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("freec holds: status=%q, want violated", resp.Verdict.Status)
	}
	var sawFree bool
	for _, w := range resp.Verdict.Witness {
		sawFree = sawFree || strings.Contains(w.Feature, "P::free")
	}
	if !sawFree {
		t.Errorf("freec witness %+v does not name P::free", resp.Verdict.Witness)
	}
	resp = verifyQuestion(t, srv, hash, "P::freec", questionSatisfiable, "", "")
	if resp.Verdict.Status != statusSatisfiable {
		t.Errorf("freec satisfiable: status=%q, want satisfiable: %q", resp.Verdict.Status, resp.Verdict.Error)
	}

	resp = verifyQuestion(t, srv, hash, "P::Tank::s", questionHolds, "P::tank", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("Tank::s holds on tank: status=%q, want violated", resp.Verdict.Status)
	}
	var sawSpare bool
	for _, w := range resp.Verdict.Witness {
		sawSpare = sawSpare || strings.Contains(w.Feature, "P::Tank::spare")
	}
	if !sawSpare {
		t.Errorf("Tank::s witness %+v does not name P::Tank::spare", resp.Verdict.Witness)
	}
}

const satisfactionChainModelSource = `package P {
	private import ScalarValues::*;
	part def Inner { attribute power : Real; }
	part def Sub { attribute power : Real; part inner : Inner; }
	part def Thing { part sub : Sub; }
	requirement def R2 { subject vehicle : Thing; require constraint { vehicle.sub.power > 0.0 } }
	requirement def R3 { subject vehicle : Thing; require constraint { vehicle.sub.inner.power > 0.0 } }
	requirement def R4 { subject vehicle : Thing; require constraint { vehicle.sub.inner.power > 8.0 } }
	requirement r2 : R2;
	requirement r3 : R3;
	requirement r4 : R4;
	part craft : Thing {
		part :>> sub {
			attribute :>> power = 5.0;
			part :>> inner { attribute :>> power = 7.0; }
		}
	}
	part analysis {
		assert satisfy r2 by craft;
		assert satisfy r3 by craft;
		assert satisfy r4 by craft;
	}
}
`

func TestVerifyQuestionsSatisfactionChainsReadTheSubject(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, satisfactionChainModelSource, "verify-satisfaction-chain-values")
	want := map[string]string{
		"satisfy r2 by craft": "vehicle.sub.power",
		"satisfy r3 by craft": "vehicle.sub.inner.power",
		"satisfy r4 by craft": "vehicle.sub.inner.power",
	}
	read := func(question string) map[string]*pb.Verdict {
		t.Helper()
		resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
			ModelHash: hash, Question: question,
		})
		if err != nil {
			t.Fatalf("VerifySatisfaction(%s): %v", question, err)
		}
		verdicts := make(map[string]*pb.Verdict)
		for _, v := range resp.Verdicts {
			for element := range want {
				if strings.Contains(v.Element, element) {
					verdicts[element] = v
				}
			}
		}
		return verdicts
	}

	evaluated := read(questionEvaluate)
	held := read(questionHolds)
	satisfiable := read(questionSatisfiable)
	for element, chain := range want {
		wantEvaluation := statusHolds
		if element == "satisfy r4 by craft" {
			wantEvaluation = statusViolated
		}
		if evaluated[element] == nil || evaluated[element].Status != wantEvaluation {
			t.Errorf("%s evaluate = %v, want %s", element, evaluated[element], wantEvaluation)
		}
		if held[element] == nil {
			t.Errorf("%s holds question returned no verdict", element)
			continue
		}
		if element == "satisfy r4 by craft" {
			if held[element].Status != statusViolated {
				t.Errorf("%s holds = %q, want violated (the held value is 7.0)",
					element, held[element].Status)
			}
			for _, w := range held[element].Witness {
				if strings.Contains(w.Feature, chain) {
					t.Errorf("%s witness includes pinned chain value %+v", element, w)
				}
			}
			continue
		}
		if held[element].Status != statusHolds {
			t.Errorf("%s holds = %q error=%q, want holds", element, held[element].Status, held[element].Error)
		}
		if satisfiable[element] == nil || satisfiable[element].Status != statusSatisfiable {
			t.Errorf("%s satisfiable = %v, want satisfiable", element, satisfiable[element])
			continue
		}
		for _, w := range satisfiable[element].Witness {
			if strings.Contains(w.Feature, chain) {
				t.Errorf("%s satisfiable witness includes pinned chain value %+v", element, w)
			}
		}
	}
}
