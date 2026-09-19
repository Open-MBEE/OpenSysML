package smt

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// EngineName is the name the engine registers under.
const EngineName = analysis.SMTEngineName

// DefaultMoves is the move bound k when the budget names no depth.
const DefaultMoves = 40

// Engine is the `smt` analysis engine over the solver discover finds.
type Engine struct {
	discover func() (*solve.Solver, error)
}

// New returns the engine over the solver discover finds, nil discovering as
// solve.Discover does.
func New(discover func() (*solve.Solver, error)) *Engine {
	if discover == nil {
		discover = solve.Discover
	}
	return &Engine{discover: discover}
}

// Name is `smt`.
func (*Engine) Name() string { return EngineName }

// Describe: every schedule of at most k moves over the inputs free in their declared
// domains, decided by a solver; an `unsat` with no bound reachable is a proof, a
// witness is replayed before it is claimed. A Sensitive question is decided over two
// copies of the schedule sharing the initial state, both witnesses replayed.
func (*Engine) Describe() analysis.Description {
	return analysis.Description{
		Questions: []analysis.Kind{analysis.Holds, analysis.Sensitive},
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

// Covers takes a Holds or Sensitive question with the schedule free, its inputs free or
// as written; a fixed schedule, another kind and a question without its ask are refused.
func (e *Engine) Covers(_ *analysis.Model, q analysis.Question) analysis.Coverage {
	if q.Kind != analysis.Holds && q.Kind != analysis.Sensitive {
		return analysis.Coverage{Refusal: &analysis.NotAskedError{Engine: e.Name(), Kind: q.Kind}}
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
	unroll   int
	encoding *Encoding
	property *Property
	deadlock *Property
	// compared tells whose feature each name a Sensitive question compares is, and
	// performer names the performing object's attributes, compared absent names.
	compared  map[string]runtime.FeatureOwner
	performer []string
	started   time.Time
	timedOut  bool
}

// Run encodes the flow for Budget.Depth moves (DefaultMoves when none), body loops unrolled
// Budget.Unroll times (DefaultUnroll when none), and asks, in order, for a violation, a
// failure and a deadlock; a `sat` is replayed before it is claimed. A Sensitive question
// asks the two-copy query first, as decideSensitive spells.
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
	r := &run{engine: e, model: model, q: q, budget: budget, solver: solver, timeout: solver.Timeout, moves: budget.Depth, unroll: budget.Unroll, started: time.Now()}
	if r.timeout <= 0 {
		r.timeout = solve.DefaultTimeout
	}
	if r.moves <= 0 {
		r.moves = DefaultMoves
	}
	if r.unroll <= 0 {
		r.unroll = DefaultUnroll
	}
	if !modelBuilds(model) {
		return analysis.Result{}, &analysis.NoRuntimeError{Engine: e.Name()}
	}
	if refusal, err := r.encode(); err != nil {
		return analysis.Result{}, err
	} else if refusal != nil {
		return r.uncovered(refusal.Error()), nil
	}
	if q.Kind == analysis.Sensitive {
		return r.decideSensitive(ctx)
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
	if r.q.Kind == analysis.Sensitive {
		if r.compared, err = runtime.ResolveCheckFeatures(exec, r.q.Holds.Diverge); err != nil {
			return nil, err
		}
		r.performer = exec.PerformerAttributes()
	}
	encoding, err := Encode(ctx, r.q.Holds.Behavior, exec.Graph(), exec.Held(), r.q.Holds.Inputs, r.moves, r.unroll)
	if err != nil {
		return refusalOf(err)
	}
	for _, assumption := range r.q.Holds.Assume {
		if err := encoding.Assume(ctx, assumption, r.q.Holds.Scope); err != nil {
			return refusalOf(err)
		}
	}
	r.encoding = encoding
	r.deadlock = encoding.Deadlock()
	if len(r.q.Holds.Conditions) == 0 {
		r.property = r.deadlock
		return nil, nil
	}
	property, err := encoding.Conditions(ctx, r.q.Holds.Conditions, r.q.Holds.Scope)
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
		errors.Is(err, solve.ErrNotTranslatable), errors.Is(err, runtime.ErrNoConditions),
		errors.Is(err, analysis.ErrInput), errors.Is(err, analysis.ErrDomain):
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

// NoInitialState is the reason a run claims nothing when its assumptions admit
// no initial state: a property over no run is vacuous, not proved.
const NoInitialState = "assumptions admit no initial state"

// decide asks the queries in order, stopping at the first `sat` whose witness
// replays or at the first answer that decides nothing. An `unsat` over arithmetic
// the interpreter rounds refutes nothing, so once every query is `unsat` it
// leaves the question not covered rather than held, and assumptions no exact
// initial state satisfies are contradictory only when nothing in them rounds.
func (r *run) decide(ctx context.Context) (analysis.Result, error) {
	if out, err := r.consistent(ctx); out != nil || err != nil {
		return orEmpty(out), err
	}
	found, rounded, err := r.findings(ctx, r.asks())
	if found != nil || err != nil {
		return orEmpty(found), err
	}
	if rounded != nil {
		return *rounded, nil
	}
	return r.completes(ctx,
		func(cut Cut, _ *solve.Result) (analysis.Result, error) { return r.holds(analysis.Bounded, cut), nil },
		func() analysis.Result { return r.holds(analysis.Proved, Cut{}) })
}

// orEmpty is the result pointed at, or the zero result for nil.
func orEmpty(out *analysis.Result) analysis.Result {
	if out == nil {
		return analysis.Result{}
	}
	return *out
}

// consistent asks whether the assumptions admit an initial state, when any are
// assumed: the answer when they do not or the solver did not decide, nil otherwise.
func (r *run) consistent(ctx context.Context) (*analysis.Result, error) {
	if len(r.encoding.Assumptions) == 0 {
		return nil, nil
	}
	consistency := r.encoding.Consistency()
	result, err := r.solver.Solve(ctx, consistency)
	if err != nil {
		return nil, err
	}
	var out analysis.Result
	switch result.Status {
	case solve.StatusUnsat:
		if consistency.Rounded() {
			out = r.rounded(result, "whether the assumptions admit an initial state")
		} else {
			out = r.uncovered(NoInitialState)
		}
	case solve.StatusSat:
		return nil, nil
	default:
		out = r.undecided(result, "whether the assumptions admit an initial state")
	}
	return &out, nil
}

// ask is one query for a finding and the outcome its `sat` witnesses.
type ask struct {
	query   *solve.Query
	outcome outcome
}

// asks are the finding queries in the order decide asks them: a violation of the
// property, a failure, then a deadlock when the property is another.
func (r *run) asks() []ask {
	asks := []ask{{r.encoding.Violation(r.property), outcomeViolation}, {r.encoding.Failure(r.property), outcomeFailure}}
	if r.property == r.deadlock {
		asks[0].outcome = outcomeDeadlock
	} else {
		asks = append(asks, ask{r.encoding.Violation(r.deadlock), outcomeDeadlock})
	}
	return asks
}

// findings asks each query in order: the first `sat` replayed is found, as is the
// first answer the solver did not decide; rounded is the first `unsat` that decides
// nothing for rounding, kept while the later queries are asked.
func (r *run) findings(ctx context.Context, asks []ask) (found, rounded *analysis.Result, err error) {
	for _, a := range asks {
		result, err := r.solver.Solve(ctx, a.query)
		if err != nil {
			return nil, nil, err
		}
		switch result.Status {
		case solve.StatusUnsat:
			if rounded == nil && a.query.Rounded() {
				out := r.rounded(result, "whether a schedule reaches "+a.outcome.String())
				rounded = &out
			}
		case solve.StatusSat:
			out, err := r.witnessed(result, a.outcome)
			if err != nil {
				return nil, nil, err
			}
			return &out, rounded, nil
		default:
			out := r.undecided(result, "whether a schedule reaches "+a.outcome.String())
			return &out, rounded, nil
		}
	}
	return nil, rounded, nil
}

// completes asks whether every schedule ends within the bounds: proved answers an
// `unsat` that rounds nothing, bounded a `sat` with the bounds its model reached.
func (r *run) completes(ctx context.Context, bounded func(Cut, *solve.Result) (analysis.Result, error), proved func() analysis.Result) (analysis.Result, error) {
	uncertainty := r.encoding.Uncertainty()
	result, err := r.solver.Solve(ctx, uncertainty)
	if err != nil {
		return analysis.Result{}, err
	}
	switch result.Status {
	case solve.StatusUnsat:
		if uncertainty.Rounded() {
			return r.rounded(result, "whether every schedule ends within the bounds"), nil
		}
		return proved(), nil
	case solve.StatusSat:
		cut, err := r.encoding.Cuts(result)
		if err != nil {
			return analysis.Result{}, err
		}
		return bounded(cut, result)
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
		result.Inputs = r.inputs(nil)
		result.Assumptions = r.encoding.Assumptions
		if r.frees() {
			result.Question.Free |= analysis.FreeInputs
		}
	} else {
		result.Bounds = analysis.Bounds{{Name: "moves", Limit: int64(r.moves)}, {Name: "unroll", Limit: int64(r.unroll)},
			{Name: "solver", Limit: r.timeout.Milliseconds()}}
	}
	return result
}

// frees reports whether the encoding ranged over any input, which the answer's
// question then says it did.
func (r *run) frees() bool {
	for _, in := range r.encoding.Inputs {
		if in.Free {
			return true
		}
	}
	return false
}

// inputs lists the features the encoding ranged over or pinned, a free one's
// value the one the witness chose when there is one.
func (r *run) inputs(witness []runtime.InputTaken) []analysis.Input {
	chosen := make(map[string]string, len(witness))
	for _, in := range witness {
		chosen[in.Feature] = in.Written
	}
	inputs := make([]analysis.Input, 0, len(r.encoding.Inputs))
	for _, in := range r.encoding.Inputs {
		out := analysis.Input{Name: in.Name, Type: in.Type, Sort: in.Var.Sort.Name, Domain: in.Domain, Free: in.Free, Optional: in.Optional}
		switch {
		case in.Free:
			out.Value = chosen[in.Name]
		case in.Value.Kind != runtime.ValInvalid:
			out.Value = runtime.FormatValue(in.Value)
		}
		inputs = append(inputs, out)
	}
	return inputs
}

// uncovered claims nothing, for the reason.
func (r *run) uncovered(reason string) analysis.Result {
	result := r.result()
	result.Claim, result.Strength, result.Reason = analysis.ClaimNone, analysis.NotCovered, reason
	return result
}

// rounded is the answer to an `unsat` asking what over exact reals where the
// interpreter rounds: a run may reach what no exact schedule does, with no
// witness to replay, so the `unsat` decides nothing about the run.
func (r *run) rounded(result *solve.Result, what string) analysis.Result {
	out := r.uncovered(what + " rounds in floating point when evaluated, which an exact-real unsat does not decide")
	out.Values = []analysis.Evaluation{{Name: what, Solved: result}}
	return out
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

// witnessed decodes a `sat` and replays it: the claim stands, and the witness is
// written, only when the interpreter reaches the outcome claimed by the step named.
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
	witness := &analysis.Witness{Schedule: w.policy(), Inputs: w.Inputs, Choices: w.Choices}
	if replayed.disagreement != "" {
		out := r.uncovered(replayed.disagreement)
		out.Witness = witness
		out.Inputs = r.inputs(w.Inputs)
		return out, nil
	}
	if witness.Written, err = r.write(w, replayed); err != nil {
		return analysis.Result{}, err
	}
	out := r.result()
	out.Claim, out.Strength, out.Witness = analysis.ClaimViolated, analysis.Witnessed, witness
	out.Inputs = r.inputs(w.Inputs)
	out.Values = []analysis.Evaluation{{Name: r.property.Name, Err: replayed.err}}
	out.Reason = replayed.describe()
	return out, nil
}

// write writes the witness as a file the replay policy reads, when the question
// names a directory: its inputs and choices, the trace the replay left, and the
// property or failure it claims. It returns the path, "" when none was written.
func (r *run) write(w *Witness, p replayed) (string, error) {
	if r.q.Holds.WitnessDir == "" {
		return "", nil
	}
	file := runtime.Witness{Inputs: w.Inputs, Choices: w.Choices, Trace: p.trace}
	var violation *runtime.ViolationError
	switch {
	case errors.As(p.err, &violation):
		file.Property = violation.Element
	case p.err != nil:
		file.Fails = p.err.Error()
	}
	path := filepath.Join(r.q.Holds.WitnessDir, analysis.ViolationFile(r.q.Subject, r.q.Holds.Performer, 1))
	if err := analysis.WriteWitness(path, file.String()); err != nil {
		return "", err
	}
	return path, nil
}

// replayed is what the interpreter did under a witness: the outcome it reached,
// the error that is that outcome, the trace it left, and the disagreement when
// it reached another outcome.
type replayed struct {
	outcome      outcome
	step         int
	err          error
	trace        string
	disagreement string
}

// describe is the outcome as the report prints it.
func (p replayed) describe() string {
	return fmt.Sprintf("at step %d: %v", p.step, p.err)
}

// replay runs the behavior afresh under the witness, one step at a time, and
// compares what the interpreter reaches with what the witness claims, tracing
// the run so the witness file records what a replay of it must leave.
func (r *run) replay(w *Witness, expected outcome) (replayed, error) {
	ctx, err := r.model.NewContextOn(0, r.budget)
	if err != nil {
		return replayed{}, err
	}
	if err := ctx.SetSchedule(w.policy()); err != nil {
		return replayed{}, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(runtime.NewTraceRecorder())
	}
	p, err := r.follow(ctx, w, expected)
	if err == nil {
		p.trace = ctx.Trace().String()
	}
	return p, err
}

// follow steps the behavior in ctx under the witness's schedule up to the state
// it marks, judging each step against the outcome claimed.
func (r *run) follow(ctx *runtime.Context, w *Witness, expected outcome) (replayed, error) {
	claim := fmt.Sprintf("the solver claims %v at move %d", expected, w.Mark)
	disagree := func(format string, args ...any) (replayed, error) {
		return replayed{disagreement: claim + "; " + fmt.Sprintf(format, args...)}, nil
	}
	exec, err := r.q.Holds.Start(ctx)
	if err != nil {
		var refused *runtime.WitnessInputError
		if errors.As(err, &refused) {
			return disagree("the interpreter could not fix the witness's inputs: %v", err)
		}
		return replayed{}, err
	}
	defer exec.Release()
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
		for _, condition := range r.property.Conditions {
			if ok, err := exec.Holds(condition, r.q.Holds.Scope); err != nil {
				return r.reached(ctx, limit, expected, step, err, disagree)
			} else if !ok {
				return disagree("the interpreter reports %s neither holding nor violated at step %d", condition.Name, step)
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
