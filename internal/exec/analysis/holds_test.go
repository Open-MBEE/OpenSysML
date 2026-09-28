package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// holdsContext indexes src over the standard library — Holds tests resolve the
// scalar and quantity value types — and returns a runtime over it with the index.
func holdsContext(t *testing.T, src string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	sf := source.New("<test>", []byte(src))
	idx.AddDocument("<test>", parser.New(sf).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(passes.NewTypedModel(resolver), resolver), 10000)
	ctx.Model().RegisterSource(sf)
	return ctx, idx
}

// holdsQueries are the violation queries of the named constraints, each
// translated against the package as scope.
func holdsQueries(t *testing.T, src string, names ...string) []*solve.Query {
	t.Helper()
	ctx, idx := holdsContext(t, src)
	queries := make([]*solve.Query, 0, len(names))
	for _, name := range names {
		matches := idx.LookupQualified(name)
		if len(matches) != 1 {
			t.Fatalf("%s matched %d symbols, want 1", name, len(matches))
		}
		q, err := solve.ConstraintViolation(ctx, matches[0], nil, nil)
		if err != nil {
			t.Fatalf("translate %s: %v", name, err)
		}
		queries = append(queries, q)
	}
	return queries
}

// prove asks a Holds question of the queries and fails the test on a fault.
func prove(t *testing.T, queries []*solve.Query) Plan {
	t.Helper()
	plan, err := Default().Prove(context.Background(), Request{Subject: "test", Selection: Auto()}, queries)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	return plan
}

// TestHoldsAllHoldingQueriesProve: every violation query unsat — proved, when
// the conditions round — proves the claims hold over every assignment of the
// question's free inputs.
func TestHoldsAllHoldingQueriesProve(t *testing.T) {
	requireSolver(t)
	queries := holdsQueries(t, `
		package P {
			private import ScalarValues::*;
			part hg { attribute efficiency : Real; attribute power : Real; }
			attribute d : Real;
			assert constraint lemma {
				(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
				implies (hg.power * d * hg.efficiency) <= hg.power * d
			}
		}
	`, "P::lemma")
	plan := prove(t, queries)
	if plan.Result.Claim != ClaimHolds || plan.Result.Strength != Proved {
		t.Fatalf("claim is %s %s, want the lemma proved: %s", plan.Result.Claim, plan.Result.Strength, plan.Result.Reason)
	}
}

// TestHoldsViolatedWitnesses: a witnessed violation — a sat violation query —
// answers the question violated even while the other claim holds, and the
// evaluator confirms the witness as a violation.
func TestHoldsViolatedWitnesses(t *testing.T) {
	requireSolver(t)
	src := `
		package P {
			private import ScalarValues::*;
			part hg { attribute efficiency : Real; attribute power : Real; }
			attribute d : Real;
			assert constraint lemma {
				(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
				implies (hg.power * d * hg.efficiency) <= hg.power * d
			}
			assert constraint bad { hg.power * d <= hg.power }
		}
	`
	queries := holdsQueries(t, src, "P::lemma", "P::bad")
	plan := prove(t, queries)
	if plan.Result.Claim != ClaimViolated || plan.Result.Strength != Witnessed {
		t.Fatalf("claim is %s %s, want a witnessed violation", plan.Result.Claim, plan.Result.Strength)
	}
	var witnessed bool
	for _, v := range plan.Result.Values {
		if v.Solved == nil || v.Solved.Status != solve.StatusSat {
			continue
		}
		witnessed = true
		values := make(map[string]solve.ModelValue, len(v.Solved.Model))
		for _, a := range v.Solved.Model {
			value, err := solve.DecodeValue(a)
			if err != nil {
				t.Fatalf("decode %s: %v", a.Var.Name, err)
			}
			values[a.Var.Name] = value
		}
		var confirms bool
		for _, q := range queries {
			if ok, _ := q.Confirm(values); !ok {
				confirms = true
			}
		}
		if !confirms {
			t.Error("the evaluator does not confirm the witness as a violation of any queried claim")
		}
	}
	if !witnessed {
		t.Fatal("no sat value carried the violation witness")
	}
}

// TestHoldsQuantitiesProve: a claim over an attribute typed by a quantity —
// its feature resolves through the ISQ and SI library — proves like a scalar
// one, every free assignment of the quantity holding it.
func TestHoldsQuantitiesProve(t *testing.T) {
	requireSolver(t)
	queries := holdsQueries(t, `
		package P {
			private import ScalarValues::*;
			private import ISQ::*;
			private import SI::*;
			part engine { attribute power : PowerValue; assert constraint reflexive { power == power } }
		}
	`, "P::engine::reflexive")
	plan := prove(t, queries)
	if plan.Result.Claim != ClaimHolds || plan.Result.Strength != Proved {
		t.Fatalf("claim is %s %s, want proved: %s", plan.Result.Claim, plan.Result.Strength, plan.Result.Reason)
	}
}

// TestHoldsNonRealProves: claims over enum and boolean encodings never round,
// so their unsat proves holds outright.
func TestHoldsNonRealProves(t *testing.T) {
	requireSolver(t)
	queries := holdsQueries(t, `
		package P {
			private import ScalarValues::*;
			enum def Mode { on; off; }
			part light { attribute m : Mode; attribute b : Boolean; }
			assert constraint trichotomy { light.m == Mode::on or light.m == Mode::off }
			assert constraint tautology { light.b or not light.b }
		}
	`, "P::trichotomy", "P::tautology")
	plan := prove(t, queries)
	if plan.Result.Claim != ClaimHolds || plan.Result.Strength != Proved {
		t.Fatalf("claim is %s %s, want proved: %s", plan.Result.Claim, plan.Result.Strength, plan.Result.Reason)
	}
}

// TestHoldsExponentiationRefuses: exponentiation translates only under a
// definedness guard a violation query cannot hoist, so a claim using it
// refuses rather than being proved falsely.
func TestHoldsExponentiationRefuses(t *testing.T) {
	ctx, idx := holdsContext(t, `
		package P {
			private import ScalarValues::*;
			part thing { attribute x : Real; assert constraint square { x ** 2.0 > 0.0 } }
		}
	`)
	matches := idx.LookupQualified("P::thing::square")
	if len(matches) != 1 {
		t.Fatalf("square matched %d symbols, want 1", len(matches))
	}
	q, err := solve.ConstraintViolation(ctx, matches[0], nil, nil)
	if q != nil || !errors.Is(err, solve.ErrNotTranslatable) {
		t.Fatalf("translate: q=%v err=%v, want a refusal", q, err)
	}
	var refused *solve.NotTranslatableError
	if !errors.As(err, &refused) || !strings.Contains(refused.Error(), "exponentiation") {
		t.Errorf("refusal is %v, want the exponentiation reason", err)
	}
}

// TestSolveEngineCoversHolds: the solve engine covers a Holds question only
// with violation queries to ask; a non-violation query would ask the opposite
// of the same translation and is refused.
func TestSolveEngineCoversHolds(t *testing.T) {
	e := NewSolve(func() (*solve.Solver, error) { return &solve.Solver{Name: "test"}, nil })
	violation := &solve.Query{Element: "C", Violation: true}
	plain := &solve.Query{Element: "C"}
	if c := e.Covers(nil, Question{Kind: Holds, Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{violation}, Ask: (*solve.Solver).Solve}}); !c.Covered {
		t.Errorf("Holds over violation queries refused: %v", c.Refusal)
	}
	if c := e.Covers(nil, Question{Kind: Holds, Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{plain}, Ask: (*solve.Solver).Solve}}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Error("Holds over a non-violation query covered")
	}
	if c := e.Covers(nil, Question{Kind: Satisfiable, Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{violation}, Ask: (*solve.Solver).Solve}}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Error("Satisfiable over a violation query covered")
	}
	if c := e.Covers(nil, Question{Kind: Holds, Free: FreeInputs}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Error("Holds with nothing to ask covered")
	}
}

// TestCheckEngineRefusesBareHolds: the check engine answers Holds only over a
// checked behavior; a question lacking one is malformed, not a panic.
func TestCheckEngineRefusesBareHolds(t *testing.T) {
	if c := NewCheck().Covers(nil, Question{Kind: Holds, Free: FreeSchedule}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Errorf("check engine covered a Holds lacking its Check: %+v", c)
	}
}

// TestHoldsWithoutSolverErrors: with no solver the question fails, it is not
// proved.
func TestHoldsWithoutSolverErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	queries := holdsQueries(t, `
		package P {
			private import ScalarValues::*;
			part thing { attribute x : Integer; assert constraint positive { x + 1 > x } }
		}
	`, "P::thing::positive")
	_, err := Default().Prove(context.Background(), Request{Subject: "test", Selection: Auto()}, queries)
	var absent *ProcessAbsentError
	if !errors.As(err, &absent) {
		t.Fatalf("prove: %v, want the solver's absence", err)
	}
}
