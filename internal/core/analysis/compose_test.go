package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// holdsQuestion is a Holds question over schedules, as the smt engine will take it.
var holdsQuestion = Question{Kind: Holds, Subject: "test::Tank::low", Free: FreeSchedule}

// exploreWitness is a violation the interpreter replayed under explore, in the shape
// the explore engine's witness takes: the policy and the choices the run made.
func exploreWitness(t *testing.T) Result {
	t.Helper()
	return Result{
		Question: holdsQuestion,
		Engine:   ExploreEngineName,
		Claim:    ClaimViolated,
		Strength: Witnessed,
		Bounds:   Bounds{{Name: "runs", Limit: 1024}, {Name: "depth", Limit: 64}},
		Witness:  &Witness{Schedule: policy(t, "explore"), Choices: []runtime.ChoiceTaken{{Where: "split", Alternatives: 3, Taken: 2}}},
	}
}

// smtProof is a proof over schedules, in the shape the smt engine will fill.
var smtProof = Result{Question: holdsQuestion, Engine: "smt", Claim: ClaimHolds, Strength: Proved, Bounds: Bounds{{Name: "solver", Limit: 30000}}}

func TestComposeResolvesAContradictionInTheInterpretersFavor(t *testing.T) {
	witness := exploreWitness(t)
	for _, order := range [][]Result{{smtProof, witness}, {witness, smtProof}} {
		composed, disagreements := Compose(order)
		if composed.Engine != ExploreEngineName || composed.Claim != ClaimViolated || composed.Strength != Witnessed {
			t.Fatalf("composed %+v, want the interpreter's witness to stand", composed)
		}
		if len(disagreements) != 1 {
			t.Fatalf("disagreements %+v, want the one between smt and explore", disagreements)
		}
		d := disagreements[0]
		if d.Stands != ExploreEngineName || d.Demoted != "smt" || d.Claimed.Strength != Proved || d.Claimed.Claim != ClaimHolds {
			t.Fatalf("disagreement %+v, want smt's proof demoted under explore's witness", d)
		}
		if !strings.Contains(d.Reason, "smt claimed holds (proved)") || !strings.Contains(d.Reason, "explore witnessed violated") {
			t.Fatalf("reason %q, want both sides named", d.Reason)
		}
	}
}

func TestAllDemotesTheContradictedProofInThePlan(t *testing.T) {
	witness := exploreWitness(t)
	r := registered(t,
		fakeEngine{name: "smt", kinds: []Kind{Holds}, authority: Proved, result: smtProof},
		fakeEngine{name: ExploreEngineName, kinds: []Kind{Holds}, authority: Proved, result: witness},
	)
	plan, err := r.AnswerWith(context.Background(), nil, holdsQuestion, Budget{}, All())
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if plan.Result.Claim != ClaimViolated || plan.Result.Engine != ExploreEngineName || len(plan.Disagreements) != 1 {
		t.Fatalf("plan result %+v, disagreements %+v; want the witness and one disagreement", plan.Result, plan.Disagreements)
	}
	var smt *Step
	for i := range plan.Steps {
		if plan.Steps[i].Engine == "smt" {
			smt = &plan.Steps[i]
		}
	}
	if smt == nil || smt.Result == nil || smt.Result.Covered() || smt.Result.Claim != ClaimNone {
		t.Fatalf("smt step %+v, want its result demoted to not covered", smt)
	}
	if smt.Result.Reason != plan.Disagreements[0].Reason || !strings.Contains(smt.Result.Standing(), "not covered (smt claimed holds (proved)") {
		t.Fatalf("demoted %+v, want the disagreement as its reason", smt.Result)
	}
	if !sameBounds(smt.Result.Bounds, smtProof.Bounds) || plan.Disagreements[0].Claimed.Strength != Proved {
		t.Fatalf("demoted %+v, want its bounds kept and the claim it made kept in the disagreement", smt.Result)
	}
	if !strings.Contains(plan.Standing(), "violated (witnessed") || !strings.Contains(plan.Standing(), "smt not covered (") {
		t.Fatalf("standing %q, want the witness and the demotion", plan.Standing())
	}
}

func TestComposeTakesTheStrongestEarnedWithoutPromotion(t *testing.T) {
	observed := Result{Question: holdsQuestion, Engine: "a", Claim: ClaimHolds, Strength: Observed}
	bounded := Result{Question: holdsQuestion, Engine: "b", Claim: ClaimHolds, Strength: Bounded, Bounds: Bounds{{Name: "depth", Limit: 8, Reached: true}}}
	proved := Result{Question: holdsQuestion, Engine: "c", Claim: ClaimHolds, Strength: Proved}
	for _, tc := range []struct {
		name    string
		results []Result
		want    Result
	}{
		{"one observed", []Result{observed}, observed},
		{"two observed stay observed", []Result{observed, {Question: holdsQuestion, Engine: "z", Claim: ClaimHolds, Strength: Observed}}, observed},
		{"observed and bounded is the bounded", []Result{observed, bounded}, bounded},
		{"bounded and observed is the bounded", []Result{bounded, observed}, bounded},
		{"two bounded stay bounded", []Result{bounded, {Question: holdsQuestion, Engine: "z", Claim: ClaimHolds, Strength: Bounded}}, bounded},
		{"proved stands over the rest", []Result{observed, proved, bounded}, proved},
		{"not covered does not count", []Result{{Question: holdsQuestion, Engine: "n", Strength: NotCovered}, observed}, observed},
	} {
		composed, disagreements := Compose(tc.results)
		if composed.Engine != tc.want.Engine || composed.Strength != tc.want.Strength || composed.Claim != tc.want.Claim || len(disagreements) != 0 {
			t.Fatalf("%s: composed %+v (%d disagreements), want %+v", tc.name, composed, len(disagreements), tc.want)
		}
	}
	if composed, _ := Compose([]Result{{Strength: NotCovered}}); composed.Covered() || composed.Engine != "" {
		t.Fatalf("composed %+v, want nothing from nothing covered", composed)
	}
}

func TestComposeIsIndependentOfFinishOrder(t *testing.T) {
	a := Result{Question: holdsQuestion, Engine: "a", Claim: ClaimHolds, Strength: Bounded}
	b := Result{Question: holdsQuestion, Engine: "b", Claim: ClaimHolds, Strength: Bounded}
	first, _ := Compose([]Result{a, b})
	second, _ := Compose([]Result{b, a})
	if first.Engine != "a" || second.Engine != "a" {
		t.Fatalf("composed %s then %s, want a's by name whichever finished first", first.Engine, second.Engine)
	}
}

func TestComposeSatisfiableWitnessStandsOverUnsat(t *testing.T) {
	q := Question{Kind: Satisfiable, Subject: "test::C", Free: FreeInputs}
	sat := Result{Question: q, Engine: SolveEngineName, Claim: ClaimSatisfiable, Strength: Witnessed}
	unsat := Result{Question: q, Engine: "other", Claim: ClaimUnsatisfiable, Strength: Proved}
	composed, disagreements := Compose([]Result{unsat, sat})
	if composed.Engine != SolveEngineName || len(disagreements) != 1 || disagreements[0].Demoted != "other" {
		t.Fatalf("composed %+v, disagreements %+v; want the witness over the proof", composed, disagreements)
	}
}

func TestComposeDifferingValuesAreASensitivity(t *testing.T) {
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy}
	one := Result{Question: q, Engine: "a", Claim: ClaimValue, Strength: Observed, Values: []Evaluation{{Name: "x", Value: intOf(1)}}}
	two := Result{Question: q, Engine: "b", Claim: ClaimValue, Strength: Observed, Values: []Evaluation{{Name: "x", Value: intOf(2)}}}
	same := Result{Question: q, Engine: "c", Claim: ClaimValue, Strength: Observed, Values: []Evaluation{{Name: "x", Value: intOf(1)}}}
	composed, disagreements := Compose([]Result{two, one})
	if composed.Claim != ClaimSensitive || composed.Strength != Witnessed || len(disagreements) != 0 {
		t.Fatalf("composed %+v (%d disagreements), want a witnessed sensitivity and no disagreement", composed, len(disagreements))
	}
	if composed.Engine != "a, b" || !strings.Contains(composed.Reason, "a gave x = 1") || !strings.Contains(composed.Reason, "b gave x = 2") {
		t.Fatalf("composed %+v, want both values named", composed)
	}
	if agreed, _ := Compose([]Result{same, one}); agreed.Claim != ClaimValue || agreed.Engine != "a" {
		t.Fatalf("composed %+v, want agreeing values to stand as observed", agreed)
	}
}

func TestOverclaimIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Bounded}})
	for _, selection := range []Selection{Auto(), All(), Only("weak")} {
		plan, err := r.AnswerWith(context.Background(), nil, holdsQuestion, Budget{}, selection)
		var overclaim *OverclaimError
		if !errors.As(err, &overclaim) || !errors.Is(err, ErrOverclaim) || overclaim.Engine != "weak" || overclaim.Claimed != Bounded || overclaim.Authority != Observed {
			t.Fatalf("%s: answer %v, want the typed overclaim", selection, err)
		}
		if plan.Result.Covered() || plan.Steps[0].Result != nil || plan.Steps[0].Err != err {
			t.Fatalf("%s: plan %+v, want the overclaim to stop the plan without a result", selection, plan)
		}
	}
}

func TestInconsistentResultIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "odd", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimViolated, Strength: Observed}})
	_, err := r.AnswerWith(context.Background(), nil, holdsQuestion, Budget{}, Auto())
	var inconsistent *InconsistentResultError
	if !errors.As(err, &inconsistent) || !errors.Is(err, ErrInconsistentResult) || inconsistent.Engine != "odd" {
		t.Fatalf("answer: %v, want the typed inconsistency naming odd", err)
	}
}

// sameBounds reports whether two bound lists agree.
func sameBounds(a, b Bounds) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
