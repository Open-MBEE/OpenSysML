package analysis

import (
	"context"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// SolveEngineName is the name of the engine that puts conditions to an SMT solver.
const SolveEngineName = "solve"

// SolveProcess is what the solve engine needs, as its description names it.
const SolveProcess = "an SMT solver: the one " + solve.SolverEnv + " names, else z3 or cvc5 on PATH"

// solveEngine answers Satisfiable and Holds questions with an SMT solver run as a process.
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
// package replays through the evaluator before it reports it. A Holds question
// asks the violation queries: an unsat (a proved one when the conditions round)
// proves the claim everywhere the question left free, a sat witnesses a violation.
func (solveEngine) Describe() Description {
	return Description{
		Questions: []Kind{Satisfiable, Holds},
		Process:   SolveProcess,
		Bounds:    []string{"runs", "solver"},
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

// Covers takes a Satisfiable or Holds question with queries to ask, over inputs
// left free and no schedule, when a solver is found. A Satisfiable question asks
// only queries for satisfying assignments, a Holds question only violation
// queries, since the two ask opposite things of the same translation.
func (e solveEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Satisfiable && q.Kind != Holds {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free.Has(FreeSchedule) {
		return refused(&FreedomError{Engine: e.Name(), Free: FreeSchedule})
	}
	if q.Solve == nil || len(q.Solve.Queries) == 0 || q.Solve.Ask == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Solve with Queries and an Ask"})
	}
	for _, query := range q.Solve.Queries {
		switch {
		case q.Kind == Satisfiable && query.Violation:
			return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "queries for satisfying assignments, not violations"})
		case q.Kind == Holds && !query.Violation:
			return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "violation queries"})
		}
	}
	if _, err := e.Process(); err != nil {
		return refused(err)
	}
	return covered
}

// Run asks the queries in turn, each under the budget's solver time and no more of them than
// its runs (every query without one), and judges the set: satisfiable when every query is,
// unsatisfiable when any is, not covered while any is undecided or unasked.
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
	runs, asked := budget.Runs, len(q.Solve.Queries)
	switch {
	case runs <= 0:
		runs = asked
	case runs < asked:
		asked = runs
	}
	started := time.Now()
	values := make([]Evaluation, len(q.Solve.Queries))
	for i, query := range q.Solve.Queries {
		values[i] = Evaluation{Name: query.Element}
		if i < asked {
			values[i].Solved, values[i].Err = q.Solve.Ask(solver, ctx, query)
		}
	}
	result := Result{
		Question: q,
		Engine:   e.Name(),
		Values:   values,
		Elapsed:  time.Since(started),
	}
	if q.Kind == Holds {
		result.Claim, result.Strength, result.Reason = judgeHeld(values, asked)
	} else {
		result.Claim, result.Strength, result.Reason = judgeSolved(values, asked)
	}
	result.Bounds = Bounds{
		{Name: "runs", Limit: int64(runs), Reached: asked < len(values)},
		{Name: "solver", Limit: timeout.Milliseconds(), Reached: anyTimedOut(values)},
	}
	return result, nil
}

// judgeSolved is the claim the answers support as a set: an undecided query, or one past
// the asked queries, leaves it not covered, an unsatisfiable one makes it so, and otherwise
// every query has a witness.
func judgeSolved(values []Evaluation, asked int) (Claim, Strength, string) {
	claim, strength := ClaimSatisfiable, Witnessed
	for i, v := range values {
		if i >= asked {
			return ClaimNone, NotCovered, v.Name + " was left unasked by the runs budget"
		}
		c, s, reason := judgeOne(v, Satisfiable)
		switch {
		case s == NotCovered:
			return ClaimNone, NotCovered, reason
		case c == ClaimUnsatisfiable, c == ClaimUnbounded && claim == ClaimSatisfiable:
			claim, strength = c, s
		}
	}
	return claim, strength, ""
}

// judgeHeld is the claim a Holds question's answers support as a set: an
// undecided query, or one past the asked queries, leaves it not covered, a
// witnessed violation makes it violated, and every query unsat proves the claim
// holds over every assignment of the free features.
func judgeHeld(values []Evaluation, asked int) (Claim, Strength, string) {
	violated := false
	uncovered := ""
	for i, v := range values {
		if i >= asked {
			if uncovered == "" {
				uncovered = v.Name + " was left unasked by the runs budget"
			}
			continue
		}
		c, s, reason := judgeOne(v, Holds)
		switch {
		case c == ClaimViolated:
			violated = true
		case s == NotCovered && uncovered == "":
			uncovered = reason
		}
	}
	// A witnessed violation refutes the claim whatever the other queries say.
	if violated {
		return ClaimViolated, Witnessed, ""
	}
	if uncovered != "" {
		return ClaimNone, NotCovered, uncovered
	}
	return ClaimHolds, Proved, ""
}

// judgeOne grades one query's answer under the question's kind: for Satisfiable
// an unsat proves over the encoded fragment and sat is the witness the solve
// package confirmed; for Holds a proved unsat is the claim holding and sat —
// replay-confirmed by Solve — is a witnessed violation. An unsat over conditions
// the evaluator rounds decides neither kind unless the rounding-sound recheck
// proved it.
func judgeOne(v Evaluation, kind Kind) (Claim, Strength, string) {
	switch {
	case v.Err != nil:
		return ClaimNone, NotCovered, v.Err.Error()
	case v.Solved == nil:
		return ClaimNone, NotCovered, v.Name + " was not put to the solver"
	}
	switch v.Solved.Status {
	case solve.StatusUnsat:
		if v.Solved.Query.Rounded() && !v.Solved.RoundingProved {
			if kind == Holds {
				return ClaimNone, NotCovered, v.Name + " holds over exact reals, but the evaluator's floating-point arithmetic was not proved to agree"
			}
			return ClaimNone, NotCovered, v.Name + " rounds in floating point when evaluated, which an exact-real unsat does not decide"
		}
		if kind == Holds {
			return ClaimHolds, Proved, ""
		}
		return ClaimUnsatisfiable, Proved, ""
	case solve.StatusUnknown:
		reason := v.Solved.Reason
		if reason == "" {
			reason = "the solver did not decide " + v.Name
		}
		return ClaimNone, NotCovered, reason
	}
	if kind == Holds {
		return ClaimViolated, Witnessed, ""
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
