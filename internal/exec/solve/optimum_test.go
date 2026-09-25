package solve

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// optimized asks the installed solver for an analysis case's optima.
func optimized(t *testing.T, name string) *Result {
	t.Helper()
	solver := requireSolver(t)
	q := analysisQuery(t, name)
	result, err := solver.Optimize(context.Background(), q)
	if err != nil {
		if errors.Is(err, ErrNoOptimization) {
			t.Skipf("%s implements no optimization: %v", solver.Name, err)
		}
		t.Fatalf("optimize %s: %v\nscript:\n%s", name, err, Script(q))
	}
	return result
}

// oneOptimum is the single optimum of a satisfiable case.
func oneOptimum(t *testing.T, name string) Optimum {
	t.Helper()
	result := optimized(t, name)
	if result.Status != StatusSat {
		t.Fatalf("%s answered %s, want sat", name, result.Status)
	}
	if len(result.Optima) != 1 {
		t.Fatalf("%s reported %d optima, want 1", name, len(result.Optima))
	}
	return result.Optima[0]
}

// TestOptimumOfAQuantity: the least mass the budget permits, reported with the
// base units the conditions express masses in.
func TestOptimumOfAQuantity(t *testing.T) {
	got := oneOptimum(t, "MassBudget")
	if got.Status != OptimumAttained {
		t.Fatalf("optimum is %s: %s", got.Status, got.Detail)
	}
	if got.Value != "10000.0 [gram]" {
		t.Errorf("least mass is %q, want 10000.0 [gram] (10 kg)", got.Value)
	}
}

// TestOptimumOfAnInteger: the greatest crew the assumption and the objective's
// own condition together permit.
func TestOptimumOfAnInteger(t *testing.T) {
	got := oneOptimum(t, "CrewSizing")
	if got.Status != OptimumAttained || got.Value != "7" {
		t.Errorf("greatest crew is %s %q: %s", got.Status, got.Value, got.Detail)
	}
}

// TestOptimumBoundedByItsDefinition: the condition the model's own objective
// definition states bounds the optimum — without it the value is unbounded below.
func TestOptimumBoundedByItsDefinition(t *testing.T) {
	got := oneOptimum(t, "BoundedByItsDefinition")
	if got.Status != OptimumAttained || got.Value != "2" {
		t.Errorf("least mass is %s %q: %s", got.Status, got.Value, got.Detail)
	}
	// The case's own `best <= 1` would contradict the definition's `best >= 2`
	// were the two one value.
	got = oneOptimum(t, "ShadowedBest")
	if got.Status != OptimumAttained || got.Value != "2" {
		t.Errorf("least mass under a shadowed best is %s %q: %s", got.Status, got.Value, got.Detail)
	}
	// The case's `weight == 100` is not the objective's `weight == mass + 1`.
	got = oneOptimum(t, "OwnWeight")
	if got.Status != OptimumAttained || got.Value != "2" {
		t.Errorf("least own weight is %s %q: %s", got.Status, got.Value, got.Detail)
	}
}

// TestOptimumOverAVariantSelection: the objective's value depends on which
// variant is chosen, and the model reported names the choice attaining it.
func TestOptimumOverAVariantSelection(t *testing.T) {
	result := optimized(t, "WheelChoice")
	if result.Status != StatusSat {
		t.Fatalf("WheelChoice answered %s, want sat", result.Status)
	}
	got := result.Optima[0]
	if got.Status != OptimumAttained || got.Value != "4" {
		t.Fatalf("lightest rim is %s %q: %s", got.Status, got.Value, got.Detail)
	}
	// 4 is the carbon rim's value, so that is the configuration reported.
	for _, a := range result.Model {
		if strings.HasSuffix(a.Var.Name, "wheel.rim") && !strings.Contains(a.Value, "carbon") {
			t.Errorf("the rim attaining the optimum is %q, want the carbon variant", a.Value)
		}
	}
}

// TestOptimumWithGuardedDivision: a case whose conditions divide by a computed
// divisor optimizes over exactly the values the evaluator would accept.
func TestOptimumWithGuardedDivision(t *testing.T) {
	got := oneOptimum(t, "GuardedRatio")
	// parts <= 4 and total <= 40 with total / parts >= 2 permit 4 parts (8 <= 40).
	if got.Status != OptimumAttained || got.Value != "4" {
		t.Errorf("most parts is %s %q: %s", got.Status, got.Value, got.Detail)
	}
}

// TestLexicographicOptima: the objectives are improved in declaration order —
// the least cost first, and the greatest margin among the assignments achieving
// it, which is not the greatest margin the case permits.
func TestLexicographicOptima(t *testing.T) {
	result := optimized(t, "CostThenMargin")
	if result.Status != StatusSat {
		t.Fatalf("CostThenMargin answered %s, want sat", result.Status)
	}
	if len(result.Optima) != 2 {
		t.Fatalf("reported %d optima, want 2", len(result.Optima))
	}
	cost, margin := result.Optima[0], result.Optima[1]
	if cost.Status != OptimumAttained || cost.Value != "3" {
		t.Errorf("least cost is %s %q: %s", cost.Status, cost.Value, cost.Detail)
	}
	// margin <= cost * 2, so the least cost bounds it at 6 rather than 18.
	if margin.Status != OptimumAttained || margin.Value != "6" {
		t.Errorf("greatest margin at the least cost is %s %q: %s",
			margin.Status, margin.Value, margin.Detail)
	}
}

// TestUnboundedOptimum: an objective the conditions do not bound has no optimum,
// which is its own answer rather than a number.
func TestUnboundedOptimum(t *testing.T) {
	got := oneOptimum(t, "UnboundedLoad")
	if got.Status != OptimumUnbounded {
		t.Fatalf("optimum is %s %q: %s", got.Status, got.Value, got.Detail)
	}
	if got.Value != "" || got.Bound != "" {
		t.Errorf("an unbounded objective reported the value %q and the bound %q", got.Value, got.Bound)
	}
	if got.Feasible == "" {
		t.Error("no feasible value reported for an unbounded objective")
	}
}

// TestBoundThatIsNotAttained: an objective approaching a bound no assignment
// attains reports the bound as one, never as an optimum. Which of the two
// non-optimum answers a backend gives depends on how it reports the bound —
// z3 4.8.12 reports a finite value verification then refutes — but neither
// presents a value as the optimum.
func TestBoundThatIsNotAttained(t *testing.T) {
	got := oneOptimum(t, "OpenMargin")
	switch got.Status {
	case OptimumBounded, OptimumUnverified, OptimumUndecided:
	default:
		t.Fatalf("optimum is %s %q: %s", got.Status, got.Value, got.Detail)
	}
	if got.Value != "" {
		t.Errorf("a bound that is not attained was reported as the optimum %q", got.Value)
	}
	if got.Detail == "" {
		t.Error("no reason given for reporting no optimum")
	}
}

// TestUnsatisfiableAnalysis: a case no assignment satisfies has nothing to
// optimize over, and the verdict stays unsat rather than becoming an optimum.
func TestUnsatisfiableAnalysis(t *testing.T) {
	result := optimized(t, "ImpossibleFit")
	if result.Status != StatusUnsat {
		t.Fatalf("ImpossibleFit answered %s, want unsat", result.Status)
	}
	if len(result.Optima) != 0 {
		t.Errorf("reported optima for an unsatisfiable case: %+v", result.Optima)
	}
}

// TestOptimumIsVerifiedIndependently: every optimum reported as attained holds up
// to the questions asked of it — an assignment attains it, and nothing better is
// feasible — so an old backend's wrong answer cannot be presented as one.
func TestOptimumIsVerifiedIndependently(t *testing.T) {
	solver := requireSolver(t)
	q := analysisQuery(t, "CrewSizing")
	result, err := solver.Optimize(context.Background(), q)
	if err != nil {
		if errors.Is(err, ErrNoOptimization) {
			t.Skipf("%s implements no optimization: %v", solver.Name, err)
		}
		t.Fatalf("optimize: %v", err)
	}
	got := result.Optima[0]
	if got.Status != OptimumAttained {
		t.Skipf("this solver reported no attained optimum: %s", got.Detail)
	}
	// Asking whether a better value is feasible must be unsatisfiable, which is
	// exactly what the optimum being verified means.
	better := betterThanQuery(t, q, got)
	if res, err := solver.Solve(context.Background(), better); err != nil {
		t.Fatalf("check for a better value: %v", err)
	} else if res.Status != StatusUnsat {
		t.Errorf("a better crew than the reported optimum is %s\nscript:\n%s", res.Status, Script(better))
	}
}

// betterThanQuery builds the query asking for a strictly better value than the
// optimum reported, which is the check an optimum must fail.
func betterThanQuery(t *testing.T, q *Query, got Optimum) *Query {
	t.Helper()
	rat, ok := ratOfSexpr(sexpr{Atom: strings.Fields(got.Value)[0]})
	if !ok {
		t.Fatalf("the optimum %q is no number", got.Value)
	}
	copyQuery := *q
	copyQuery.Objectives = nil
	copyQuery.Assertions = append(append([]Assertion{}, q.Assertions...), Assertion{
		Term: better(got.Objective, ratTerm(got.Objective.Term.Sort, rat)),
		From: Provenance{Kind: "analysis", Element: q.Element, Condition: "better than the optimum"},
	})
	return &copyQuery
}
