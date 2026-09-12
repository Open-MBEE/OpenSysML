package smt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// EngineName is the name the engine registers under.
const EngineName = "smt"

// DefaultMoves is the move bound k when the budget names no depth.
const DefaultMoves = 40

// Engine is the `smt` analysis engine over the solver discover finds.
type Engine struct {
	discover func() (*solve.Solver, error)
	unroll   int
}

// New returns the engine over the solver discover finds, nil discovering as
// solve.Discover does, unrolling body loops DefaultUnroll times.
func New(discover func() (*solve.Solver, error)) *Engine {
	if discover == nil {
		discover = solve.Discover
	}
	return &Engine{discover: discover, unroll: DefaultUnroll}
}

// Unrolling returns the engine with body loops unrolled n times, n at least 1.
func (e *Engine) Unrolling(n int) *Engine {
	return &Engine{discover: e.discover, unroll: max(n, 1)}
}

// Name is `smt`.
func (*Engine) Name() string { return EngineName }

// Describe: every schedule of at most k moves on the inputs as written, decided by a
// solver; an `unsat` with no bound reachable is a proof, a witness is replayed before
// it is claimed.
func (*Engine) Describe() analysis.Description {
	return analysis.Description{
		Questions: []analysis.Kind{analysis.Holds},
		Process:   analysis.SolveProcess,
		Bounds:    []string{"moves", "unroll", "slots", "solver"},
		Replays:   true,
		Authority: analysis.Proved,
	}
}

// Process names the solver found, or its absence as the typed refusal.
func (e *Engine) Process() (string, error) {
	solver, err := e.discover()
	if err != nil {
		return "", &analysis.ProcessAbsentError{Engine: e.Name(), Process: analysis.SolveProcess, Err: err}
	}
	return solver.Name + " at " + solver.Path, nil
}

// Covers takes a Holds question over concrete inputs with the schedule free; free
// inputs, a fixed schedule, another kind and a question without its ask are refused.
func (e *Engine) Covers(_ *analysis.Model, q analysis.Question) analysis.Coverage {
	if q.Kind != analysis.Holds {
		return analysis.Coverage{Refusal: &analysis.NotAskedError{Engine: e.Name(), Kind: q.Kind}}
	}
	if q.Free.Has(analysis.FreeInputs) {
		return analysis.Coverage{Refusal: &analysis.FreedomError{Engine: e.Name(), Free: analysis.FreeInputs}}
	}
	if !q.Free.Has(analysis.FreeSchedule) {
		return analysis.Coverage{Refusal: &ScheduleError{Engine: e.Name(), Schedule: q.Schedule}}
	}
	switch {
	case q.Holds == nil:
		return analysis.Coverage{Refusal: &analysis.MalformedQuestionError{Kind: q.Kind, Missing: "a Holds ask"}}
	case q.Holds.Behavior == nil:
		return analysis.Coverage{Refusal: &analysis.MalformedQuestionError{Kind: q.Kind, Missing: "a behavior"}}
	case q.Holds.Start == nil:
		return analysis.Coverage{Refusal: &analysis.MalformedQuestionError{Kind: q.Kind, Missing: "a Start"}}
	}
	if _, err := e.Process(); err != nil {
		return analysis.Coverage{Refusal: err}
	}
	return analysis.Coverage{Covered: true}
}

// ErrScheduleFixed is the typed error for a Holds question that fixes its schedule.
var ErrScheduleFixed = errors.New("smt decides every schedule, not one")

// ScheduleError reports a question asking about one schedule of an engine that
// decides them all.
type ScheduleError struct {
	Engine   string
	Schedule runtime.SchedulePolicy
}

func (e *ScheduleError) Error() string {
	return fmt.Sprintf("%s answers over every schedule; the question fixes %s", e.Engine, e.Schedule)
}

// Is matches ErrScheduleFixed.
func (e *ScheduleError) Is(target error) bool { return target == ErrScheduleFixed }

// run is one question's state: the encoding, its property, the bounds taken.
type run struct {
	engine   *Engine
	model    *analysis.Model
	q        analysis.Question
	budget   analysis.Budget
	solver   *solve.Solver
	timeout  time.Duration
	moves    int
	encoding *Encoding
	property *Property
	deadlock *Property
	started  time.Time
	timedOut bool
}

// Run encodes the flow for Budget.Depth moves (DefaultMoves when none) and asks, in order, for a
// violation, a failure and a deadlock; a `sat` is replayed before it is claimed.
func (e *Engine) Run(ctx context.Context, model *analysis.Model, q analysis.Question, budget analysis.Budget) (analysis.Result, error) {
	if coverage := e.Covers(model, q); !coverage.Covered {
		return analysis.Result{}, coverage.Refusal
	}
	solver, err := e.discover()
	if err != nil {
		return analysis.Result{}, &analysis.ProcessAbsentError{Engine: e.Name(), Process: analysis.SolveProcess, Err: err}
	}
	if budget.Solver > 0 {
		solver.Timeout = budget.Solver
	}
	r := &run{engine: e, model: model, q: q, budget: budget, solver: solver, timeout: solver.Timeout, moves: budget.Depth, started: time.Now()}
	if r.timeout <= 0 {
		r.timeout = solve.DefaultTimeout
	}
	if r.moves <= 0 {
		r.moves = DefaultMoves
	}
	if !modelBuilds(model) {
		return analysis.Result{}, &analysis.NoRuntimeError{Engine: e.Name()}
	}
	if refusal, err := r.encode(); err != nil {
		return analysis.Result{}, err
	} else if refusal != nil {
		return r.uncovered(refusal.Error()), nil
	}
	return r.decide(ctx)
}

// modelBuilds reports whether the model builds a context of a run's own.
func modelBuilds(m *analysis.Model) bool {
	return m != nil && m.Semantics != nil && m.Fresh != nil
}

// encode starts one run to take its lowered graph and encodes it with the
// question's property; a construct outside the encoding is the refusal returned,
// a context or start that cannot be made is the error.
func (r *run) encode() (refusal error, err error) {
	ctx, err := r.model.NewContextOn(0, r.budget)
	if err != nil {
		return nil, err
	}
	exec, err := r.q.Holds.Start(ctx)
	if err != nil {
		return nil, err
	}
	defer exec.Release()
	encoding, err := Encode(ctx, r.q.Holds.Behavior, exec.Graph(), r.moves, r.engine.unroll)
	if err != nil {
		return refusalOf(err)
	}
	r.encoding = encoding
	r.deadlock = encoding.Deadlock()
	if r.q.Holds.Condition == nil {
		r.property = r.deadlock
		return nil, nil
	}
	property, err := encoding.Condition(ctx, r.q.Holds.Condition, r.q.Holds.Scope)
	if err != nil {
		return refusalOf(err)
	}
	r.property = property
	return nil, nil
}

// refusalOf sorts an encoding error: a construct the engine does not encode or
// translate, or a flow the interpreter would refuse, is a refusal; anything
// else is a fault.
func refusalOf(err error) (error, error) {
	switch {
	case errors.Is(err, ErrNotEncoded), errors.Is(err, ErrMalformedFlow), errors.Is(err, ErrSlotOverflow),
		errors.Is(err, solve.ErrNotTranslatable), errors.Is(err, runtime.ErrNoConditions):
		return err, nil
	}
	var unsupported *solve.UnsupportedCapabilityError
	if errors.As(err, &unsupported) {
		return err, nil
	}
	return nil, err
}

// outcome is what a witness claims the interpreter does at its marked state.
type outcome int

const (
	// outcomeViolation: the condition evaluates false.
	outcomeViolation outcome = iota
	// outcomeFailure: a body raises a typed error, or the condition cannot be evaluated.
	outcomeFailure
	// outcomeDeadlock: no token can act and the flow is not complete.
	outcomeDeadlock
)

func (o outcome) String() string {
	switch o {
	case outcomeViolation:
		return "a violation"
	case outcomeFailure:
		return "a failure"
	case outcomeDeadlock:
		return "a deadlock"
	}
	return "an outcome"
}

// decide asks the queries in order, stopping at the first `sat` whose witness
// replays or at the first answer that decides nothing.
func (r *run) decide(ctx context.Context) (analysis.Result, error) {
	type ask struct {
		query   *solve.Query
		outcome outcome
	}
	asks := []ask{{r.encoding.Violation(r.property), outcomeViolation}, {r.encoding.Failure(r.property), outcomeFailure}}
	if r.property == r.deadlock {
		asks[0].outcome = outcomeDeadlock
	} else {
		asks = append(asks, ask{r.encoding.Violation(r.deadlock), outcomeDeadlock})
	}
	for _, a := range asks {
		result, err := r.solver.Solve(ctx, a.query)
		if err != nil {
			return analysis.Result{}, err
		}
		switch result.Status {
		case solve.StatusUnsat:
			continue
		case solve.StatusSat:
			return r.witnessed(result, a.outcome)
		default:
			return r.undecided(result, "whether a schedule reaches "+a.outcome.String()), nil
		}
	}
	result, err := r.solver.Solve(ctx, r.encoding.Uncertainty())
	if err != nil {
		return analysis.Result{}, err
	}
	switch result.Status {
	case solve.StatusUnsat:
		return r.holds(analysis.Proved, Cut{}), nil
	case solve.StatusSat:
		cut, err := r.encoding.Cuts(result)
		if err != nil {
			return analysis.Result{}, err
		}
		return r.holds(analysis.Bounded, cut), nil
	default:
		return r.undecided(result, "whether every schedule ends within the bounds"), nil
	}
}

// noteTimeout records a solver answer that ran out of time.
func (r *run) noteTimeout(result *solve.Result) {
	if result != nil && result.TimedOut {
		r.timedOut = true
	}
}

// bounds are the bounds the run took, marking the ones cut reached.
func (r *run) bounds(cut Cut) analysis.Bounds {
	return analysis.Bounds{
		{Name: "moves", Limit: int64(r.moves), Reached: cut.Moves},
		{Name: "unroll", Limit: int64(r.encoding.Unroll), Reached: cut.Unroll},
		{Name: "slots", Limit: int64(r.encoding.Flow.Slots), Reached: cut.Slots},
		{Name: "solver", Limit: r.timeout.Milliseconds(), Reached: r.timedOut},
	}
}

// result is the shape every answer shares.
func (r *run) result() analysis.Result {
	result := analysis.Result{Question: r.q, Engine: r.engine.Name(), Elapsed: time.Since(r.started)}
	if r.encoding != nil {
		result.Bounds = r.bounds(Cut{})
	} else {
		result.Bounds = analysis.Bounds{{Name: "moves", Limit: int64(r.moves)}, {Name: "unroll", Limit: int64(r.engine.unroll)},
			{Name: "solver", Limit: r.timeout.Milliseconds()}}
	}
	return result
}

// uncovered claims nothing, for the reason.
func (r *run) uncovered(reason string) analysis.Result {
	result := r.result()
	result.Claim, result.Strength, result.Reason = analysis.ClaimNone, analysis.NotCovered, reason
	return result
}

// undecided is the answer to a query, asking what, that the solver did not decide.
func (r *run) undecided(result *solve.Result, what string) analysis.Result {
	r.noteTimeout(result)
	reason := "the solver did not decide " + what
	if result.TimedOut {
		reason = fmt.Sprintf("%s within %s", reason, r.timeout)
	} else if result.Reason != "" {
		reason += ": " + result.Reason
	}
	out := r.uncovered(reason)
	out.Values = []analysis.Evaluation{{Name: what, Solved: result}}
	return out
}

// holds claims the property over every schedule: proved when no bound was
// reachable, bounded by the ones cut names otherwise.
func (r *run) holds(strength analysis.Strength, cut Cut) analysis.Result {
	result := r.result()
	result.Claim, result.Strength, result.Bounds = analysis.ClaimHolds, strength, r.bounds(cut)
	return result
}

// witnessed decodes a `sat` and replays it: the claim stands only when the
// interpreter reaches the outcome the witness claims by the step it names.
func (r *run) witnessed(result *solve.Result, expected outcome) (analysis.Result, error) {
	w, err := r.encoding.Decode(result)
	if err != nil {
		var malformed *WitnessError
		if errors.As(err, &malformed) || errors.Is(err, ErrNoWitness) {
			return r.uncovered("the solver's witness does not decode: " + err.Error()), nil
		}
		return analysis.Result{}, err
	}
	replayed, err := r.replay(w, expected)
	if err != nil {
		return analysis.Result{}, err
	}
	witness := &analysis.Witness{Schedule: runtime.ReplayPolicy(w.Choices), Choices: w.Choices}
	if replayed.disagreement != "" {
		out := r.uncovered(replayed.disagreement)
		out.Witness = witness
		return out, nil
	}
	out := r.result()
	out.Claim, out.Strength, out.Witness = analysis.ClaimViolated, analysis.Witnessed, witness
	out.Values = []analysis.Evaluation{{Name: r.property.Name, Err: replayed.err}}
	out.Reason = replayed.describe()
	return out, nil
}

// replayed is what the interpreter did under a witness: the outcome it reached,
// the error that is that outcome, and the disagreement when it reached another.
type replayed struct {
	outcome      outcome
	step         int
	err          error
	disagreement string
}

// describe is the outcome as the report prints it.
func (p replayed) describe() string {
	return fmt.Sprintf("at step %d: %v", p.step, p.err)
}

// replay runs the behavior afresh under the witness, one step at a time, and
// compares what the interpreter reaches with what the witness claims.
func (r *run) replay(w *Witness, expected outcome) (replayed, error) {
	ctx, err := r.model.NewContextOn(0, r.budget)
	if err != nil {
		return replayed{}, err
	}
	if err := ctx.SetSchedule(runtime.ReplayPolicy(w.Choices)); err != nil {
		return replayed{}, err
	}
	exec, err := r.q.Holds.Start(ctx)
	if err != nil {
		return replayed{}, err
	}
	defer exec.Release()
	claim := fmt.Sprintf("the solver claims %v at move %d", expected, w.Mark)
	disagree := func(format string, args ...any) (replayed, error) {
		return replayed{disagreement: claim + "; " + fmt.Sprintf(format, args...)}, nil
	}
	// A deadlock is reported by the step that finds no token to move, so one at
	// the initial state is the first step's.
	limit := w.Mark
	if expected == outcomeDeadlock && limit == 0 {
		limit = 1
	}
	for step := 0; ; step++ {
		if step > 0 {
			if exec.State() == runtime.StateCompleted {
				return disagree("the interpreter completed the run after %d steps", step-1)
			}
			if err := exec.Step(); err != nil {
				var refused *runtime.ReplayError
				if errors.As(err, &refused) {
					return disagree("the interpreter could not follow the witness: %v", err)
				}
				return r.reached(ctx, limit, expected, step, err, disagree)
			}
		}
		if r.property.Condition != nil {
			if ok, err := exec.Holds(r.property.Condition, r.q.Holds.Scope); err != nil {
				return r.reached(ctx, limit, expected, step, err, disagree)
			} else if !ok {
				return disagree("the interpreter reports %s neither holding nor violated at step %d", r.property.Name, step)
			}
		}
		if step >= limit {
			return disagree("the interpreter passed step %d without %v", limit, expected)
		}
	}
}

// reached judges the outcome the interpreter's error is against the one claimed
// by the step limit.
func (r *run) reached(ctx *runtime.Context, limit int, expected outcome, step int, err error, disagree func(string, ...any) (replayed, error)) (replayed, error) {
	got := outcomeFailure
	var violation *runtime.ViolationError
	switch {
	case errors.As(err, &violation):
		got = outcomeViolation
	case errors.Is(err, runtime.ErrActionDeadlock):
		got = outcomeDeadlock
	}
	if got != expected {
		return disagree("the interpreter reached %v at step %d: %v", got, step, err)
	}
	if step > limit {
		return disagree("the interpreter reached it at step %d", step)
	}
	if left := ctx.Unfollowed(); left != nil {
		return disagree("the interpreter reached it at step %d with witness moves left: %v", step, left)
	}
	return replayed{outcome: got, step: step, err: err}, nil
}
