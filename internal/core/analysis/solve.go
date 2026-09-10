package analysis

import (
	"context"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// SolveEngineName is the name of the engine that puts conditions to an SMT solver.
const SolveEngineName = "solve"

// SolveProcess is what the solve engine needs, as its description names it.
const SolveProcess = "an SMT solver: the one " + solve.SolverEnv + " names, else z3 or cvc5 on PATH"

// solveEngine answers Satisfiable questions with an SMT solver run as a process.
type solveEngine struct {
	discover func() (*solve.Solver, error)
}

// NewSolve returns the solve engine over the solver discover finds; nil
// discovers as solve.Discover does.
func NewSolve(discover func() (*solve.Solver, error)) External {
	if discover == nil {
		discover = solve.Discover
	}
	return solveEngine{discover: discover}
}

// Name is `solve`.
func (solveEngine) Name() string { return SolveEngineName }

// Describe: unsat is a proof over every assignment, sat a witness the solve
// package replays through the evaluator before it reports it.
func (solveEngine) Describe() Description {
	return Description{
		Questions: []Kind{Satisfiable},
		Process:   SolveProcess,
		Bounds:    []string{"solver"},
		Replays:   true,
		Authority: Proved,
	}
}

// Process names the solver found, or reports its absence.
func (e solveEngine) Process() (string, error) {
	solver, err := e.discover()
	if err != nil {
		return "", &ProcessAbsentError{Engine: e.Name(), Process: SolveProcess, Err: err}
	}
	return solver.Name + " at " + solver.Path, nil
}

// Covers takes a Satisfiable question with queries to ask, over inputs left
// free and no schedule, when a solver is found.
func (e solveEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Satisfiable {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free.Has(FreeSchedule) {
		return refused(&FreedomError{Engine: e.Name(), Free: FreeSchedule})
	}
	if q.Solve == nil || len(q.Solve.Queries) == 0 || q.Solve.Ask == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Solve with Queries and an Ask"})
	}
	if _, err := e.Process(); err != nil {
		return refused(err)
	}
	return covered
}

// Run asks each query in turn under the budget's solver time and judges the set: satisfiable
// when every query is, unsatisfiable when any is, not covered while any is undecided.
func (e solveEngine) Run(ctx context.Context, _ *Model, q Question, budget Budget) (Result, error) {
	solver, err := e.discover()
	if err != nil {
		return Result{}, &ProcessAbsentError{Engine: e.Name(), Process: SolveProcess, Err: err}
	}
	if budget.Solver > 0 {
		solver.Timeout = budget.Solver
	}
	timeout := solver.Timeout
	if timeout <= 0 {
		timeout = solve.DefaultTimeout
	}
	started := time.Now()
	values := make([]Evaluation, len(q.Solve.Queries))
	for i, query := range q.Solve.Queries {
		solved, err := q.Solve.Ask(solver, ctx, query)
		values[i] = Evaluation{Name: query.Element, Solved: solved, Err: err}
	}
	result := Result{
		Question: q,
		Engine:   e.Name(),
		Values:   values,
		Elapsed:  time.Since(started),
	}
	result.Claim, result.Strength, result.Reason = judgeSolved(values)
	result.Bounds = Bounds{{Name: "solver", Limit: timeout.Milliseconds(), Reached: anyTimedOut(values)}}
	return result, nil
}

// judgeSolved is the claim the answers support as a set: an undecided query leaves it not
// covered, an unsatisfiable one makes it so, and otherwise every query has a witness.
func judgeSolved(values []Evaluation) (Claim, Strength, string) {
	claim, strength := ClaimSatisfiable, Witnessed
	for _, v := range values {
		c, s, reason := judgeOne(v)
		switch {
		case s == NotCovered:
			return ClaimNone, NotCovered, reason
		case c == ClaimUnsatisfiable, c == ClaimUnbounded && claim == ClaimSatisfiable:
			claim, strength = c, s
		}
	}
	return claim, strength, ""
}

// judgeOne grades one query's answer: unsat proves over the encoded fragment unless the
// conditions round; sat is the witness the solve package confirmed.
func judgeOne(v Evaluation) (Claim, Strength, string) {
	switch {
	case v.Err != nil:
		return ClaimNone, NotCovered, v.Err.Error()
	case v.Solved == nil:
		return ClaimNone, NotCovered, v.Name + " was not put to the solver"
	}
	switch v.Solved.Status {
	case solve.StatusUnsat:
		if v.Solved.Query.Rounded() {
			return ClaimNone, NotCovered, v.Name + " rounds in floating point when evaluated, which an exact-real unsat does not decide"
		}
		return ClaimUnsatisfiable, Proved, ""
	case solve.StatusUnknown:
		reason := v.Solved.Reason
		if reason == "" {
			reason = "the solver did not decide " + v.Name
		}
		return ClaimNone, NotCovered, reason
	}
	return judgeOptima(v)
}

// judgeOptima grades a sat answer by its objectives: unbounded proves there is no optimum,
// not attained leaves it unestablished, and attained optima stand as the witness does.
func judgeOptima(v Evaluation) (Claim, Strength, string) {
	claim, strength := ClaimSatisfiable, Witnessed
	for _, o := range v.Solved.Optima {
		switch o.Status {
		case solve.OptimumAttained:
		case solve.OptimumUnbounded:
			claim, strength = ClaimUnbounded, Proved
		default:
			return ClaimNone, NotCovered, o.Objective.Name + ": " + o.Detail
		}
	}
	return claim, strength, ""
}

// anyTimedOut reports whether any query ran out of solver time.
func anyTimedOut(values []Evaluation) bool {
	for _, v := range values {
		if v.Solved != nil && v.Solved.TimedOut {
			return true
		}
	}
	return false
}
