package analysis

import (
	"context"
	"errors"
	"testing"
)

var errFixtureRefusal = errors.New("orthogonal regions are not encoded")

// stepNames lists the plan's steps by engine.
func stepNames(plan Plan) []string {
	out := make([]string, len(plan.Steps))
	for i, step := range plan.Steps {
		out[i] = step.Engine
	}
	return out
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestAutoPicksTheStrongestCoveringEngine(t *testing.T) {
	var weakRan, strongRan int
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}, ran: &weakRan},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}, ran: &strongRan},
		fakeEngine{name: "other", kinds: []Kind{Sweep}, authority: Proved, result: Result{Claim: ClaimTable, Strength: Observed}},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if plan.Result.Engine != "strong" || plan.Result.Strength != Proved {
		t.Fatalf("answered by %s at %s, want strong proved", plan.Result.Engine, plan.Result.Strength)
	}
	if !sameNames(stepNames(plan), []string{"strong"}) || strongRan != 1 || weakRan != 0 {
		t.Fatalf("steps %v (weak ran %d, strong ran %d), want strong alone", stepNames(plan), weakRan, strongRan)
	}
}

func TestAutoNamesTheRefusalAndTheFallback(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if !sameNames(stepNames(plan), []string{"strong", "weak"}) {
		t.Fatalf("steps %v, want the refusal then the fallback", stepNames(plan))
	}
	if refusal := plan.Steps[0]; !errors.Is(refusal.Refusal, errFixtureRefusal) || refusal.Result != nil || refusal.Err != nil {
		t.Fatalf("first step %+v, want strong's refusal", refusal)
	}
	if plan.Result.Engine != "weak" || plan.Result.Strength != Observed || plan.Steps[1].Result == nil {
		t.Fatalf("result %+v, want weak's observed answer", plan.Result)
	}
	if refusals := plan.Refusals(); len(refusals) != 1 || plan.Refused() != nil {
		t.Fatalf("refusals %v, refused %v: want one refusal and a plan that ran", refusals, plan.Refused())
	}
}

func TestAutoAdvancesPastARunTimeNotCovered(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, result: Result{Strength: NotCovered, Reason: "the solver answered unknown"}},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if !sameNames(stepNames(plan), []string{"strong", "weak"}) {
		t.Fatalf("steps %v, want strong's unknown then weak", stepNames(plan))
	}
	if first := plan.Steps[0]; first.Result == nil || first.Result.Covered() || first.Result.Reason != "the solver answered unknown" || first.Refusal != nil {
		t.Fatalf("first step %+v, want strong's not-covered result kept", first)
	}
	if plan.Result.Engine != "weak" || plan.Result.Claim != ClaimHolds {
		t.Fatalf("result %+v, want weak's answer", plan.Result)
	}
}

func TestAutoKeepsTheLastNotCoveredWhenNoneAnswers(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "first", kinds: []Kind{Holds}, authority: Proved, result: Result{Strength: NotCovered, Reason: "unknown"}},
		fakeEngine{name: "second", kinds: []Kind{Holds}, authority: Observed, result: Result{Strength: NotCovered, Reason: "the witness did not replay"}},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if plan.Result.Covered() || plan.Result.Engine != "second" || plan.Result.Reason != "the witness did not replay" {
		t.Fatalf("result %+v, want second's not-covered result", plan.Result)
	}
	if plan.Refused() != nil {
		t.Fatalf("refused %v, want none: both engines ran", plan.Refused())
	}
}

func TestRunErrorStopsThePlan(t *testing.T) {
	fault := errors.New("the runtime could not be made")
	var weakRan int
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}, ran: &weakRan},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, fault: fault},
	)
	plan, err := r.Answer(context.Background(), nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if !errors.Is(err, fault) {
		t.Fatalf("answer: %v, want strong's fault", err)
	}
	if !sameNames(stepNames(plan), []string{"strong"}) || !errors.Is(plan.Steps[0].Err, fault) || weakRan != 0 {
		t.Fatalf("steps %v (weak ran %d), want strong's fault alone", stepNames(plan), weakRan)
	}
}

func TestEveryRefusalIsATypedError(t *testing.T) {
	other := errors.New("no solver on PATH")
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, refusal: other},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	err := plan.Refused()
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Kind != Holds || len(refused.Refusals) != 2 {
		t.Fatalf("refused %v, want RefusedError with both refusals", err)
	}
	if !errors.Is(err, errFixtureRefusal) || !errors.Is(err, other) {
		t.Fatalf("refused %v, want each refusal reachable through errors.Is", err)
	}
	if want := "no engine answers holds: orthogonal regions are not encoded; no solver on PATH"; err.Error() != want {
		t.Fatalf("refused %q, want %q", err.Error(), want)
	}
	if plan.Result.Covered() || plan.Result.Reason != "a refused: orthogonal regions are not encoded; b refused: no solver on PATH" {
		t.Fatalf("result %+v, want every refusal named", plan.Result)
	}
}

func TestOneRefusalReadsAsItself(t *testing.T) {
	r := registered(t, fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal})
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if err := plan.Refused(); err == nil || err.Error() != errFixtureRefusal.Error() {
		t.Fatalf("refused %v, want the one refusal's own text", err)
	}
}

func TestNoEngineForTheKindIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved})
	_, err := r.Answer(context.Background(), nil, Question{Kind: Compute, Subject: "tool"}, Budget{})
	var none *NoEngineError
	if !errors.As(err, &none) || none.Kind != Compute || !errors.Is(err, ErrNoEngine) {
		t.Fatalf("answer: %v, want NoEngineError for compute", err)
	}
}

func TestEqualAuthorityRanksByName(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}},
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}},
	)
	plan := answered(t, r, nil, Question{Kind: Holds, Subject: "R"}, Budget{})
	if plan.Result.Engine != "a" {
		t.Fatalf("answered by %s, want a: name order within one authority", plan.Result.Engine)
	}
}

func TestStrengthsRankAsTheScaleReads(t *testing.T) {
	order := []Strength{NotCovered, Observed, Witnessed, Bounded, Proved}
	for i := 1; i < len(order); i++ {
		if order[i-1] >= order[i] {
			t.Fatalf("%s ranks at or above %s", order[i-1], order[i])
		}
	}
	if (Result{Strength: NotCovered}).Covered() || !(Result{Strength: Observed}).Covered() {
		t.Fatal("covered must be every strength above not covered")
	}
	for _, s := range order {
		if s.String() == "unknown" {
			t.Fatalf("%d has no name", s)
		}
	}
	for c := ClaimNone; c <= ClaimUnbounded; c++ {
		if c.String() == "unknown" {
			t.Fatalf("claim %d has no name", c)
		}
	}
	for k := Evaluate; k <= Compute; k++ {
		if k.String() == "unknown" {
			t.Fatalf("kind %d has no name", k)
		}
	}
}
