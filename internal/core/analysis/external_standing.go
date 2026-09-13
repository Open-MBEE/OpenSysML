package analysis

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// shape is the protocol check of an answer's form: a claim and strength the framework
// pairs, and the witness kind and count the question and claim take. A break is a
// ProtocolError, which ends the session.
func (e externalEngine) shape(q Question, answer enginewire.Result) (Claim, Strength, error) {
	broke := func(format string, args ...any) (Claim, Strength, error) {
		return ClaimNone, NotCovered, &ProtocolError{Engine: e.Name(), Detail: fmt.Sprintf(format, args...)}
	}
	claim, ok := ParseClaim(answer.Claim)
	if !ok {
		return broke("the result claims %q, which is no claim", answer.Claim)
	}
	strength, ok := ParseStrength(answer.Strength)
	if !ok {
		return broke("the result claims strength %q, which is no strength", answer.Strength)
	}
	if err := Consistent(claim, strength); err != nil {
		return broke("%v", err)
	}
	if claim.Existential() && answer.Witness == nil {
		return broke("the result claims %s with no witness", claim)
	}
	if !claim.Existential() && answer.Witness != nil {
		return broke("the result claims %s with a witness, which only violated, sensitive and satisfiable take", claim)
	}
	if w := answer.Witness; w != nil {
		switch {
		case e.entry.Witness == WitnessAssignment && !w.Assignment():
			return broke("the entry declares assignment witnesses and the result's is %d schedule(s)", len(w.Schedules))
		case e.entry.Witness == WitnessSchedule && w.Assignment():
			return broke("the entry declares schedule witnesses and the result's is an assignment")
		case claim == ClaimSatisfiable && !w.Assignment():
			return broke("a satisfiable result's witness is an assignment, not %d schedule(s)", len(w.Schedules))
		case claim == ClaimSensitive && len(w.Schedules) != 2:
			return broke("a sensitive result's witness is two schedules, not %d", len(w.Schedules))
		case claim == ClaimViolated && len(w.Schedules) != 1:
			return broke("a violated result's witness is one schedule, not %d", len(w.Schedules))
		case claim == ClaimSensitive && w.Feature == "":
			return broke("a sensitive result's witness names the feature that diverges")
		case claim != ClaimSatisfiable && w.Assignment():
			return broke("a %s result's witness is a schedule, not an assignment", claim)
		}
	}
	for i, x := range answer.Executions {
		if len(x.Schedules) != 1 {
			return broke("execution %d is %d schedules, not one", i+1, len(x.Schedules))
		}
	}
	if claim.Existential() && len(answer.Executions) > 0 {
		return broke("the result claims %s with executions, which only a universal or concrete claim carries", claim)
	}
	return claim, strength, nil
}

// stand labels a shaped answer by what the host checks of it: a witness that replays and
// on which the claim evaluates as claimed earns witnessed, executions that replay and on
// which the claim holds earn observed, and everything else is not covered with the
// engine's claim kept in the reason. Only the plan's clock ending is an error.
func (e externalEngine) stand(ctx context.Context, model *Model, q Question, budget Budget, answer enginewire.Result, claim Claim, strength Strength) (Result, error) {
	result := Result{Question: q, Engine: e.Name(), Claim: ClaimNone, Strength: NotCovered, Bounds: hostBounds(answer.Bounds)}
	reported := e.reported(claim, strength, answer)
	values, err := e.hostValues(model, budget, answer.Values)
	if err != nil {
		return Result{}, err
	}
	result.Values = values
	if claim == ClaimNone {
		result.Reason = reported
		if answer.Reason != "" {
			result.Reason += ": " + answer.Reason
		}
		return result, nil
	}
	if claim.Existential() && e.entry.Witness == WitnessNone {
		result.Reason = reported + " and produces no replayable witness"
		return result, nil
	}
	var stood standing
	switch claim {
	case ClaimSatisfiable:
		stood, err = e.standAssignment(q, *answer.Witness)
	case ClaimViolated:
		stood, err = e.standViolation(ctx, model, q, budget, *answer.Witness)
	case ClaimSensitive:
		stood, err = e.standSensitivity(ctx, model, q, budget, *answer.Witness)
	default:
		stood, err = e.standExecutions(ctx, model, q, budget, claim, answer)
	}
	if err != nil {
		return Result{}, err
	}
	if stood.err != nil {
		result.Reason = reported + "; " + stood.err.Error()
		return result, nil
	}
	result.Claim, result.Strength, result.Reason = claim, stood.strength, stood.reason
	result.Witness, result.Contrast, result.Executions = stood.witness, stood.contrast, stood.executions
	if len(stood.values) > 0 {
		result.Values = stood.values
	}
	return result, nil
}

// standing is what a check of the engine's evidence earned: the strength, the witnesses
// as the framework holds them and the reason, or the typed failure of the check.
type standing struct {
	strength   Strength
	reason     string
	witness    *Witness
	contrast   *Witness
	executions []Witness
	values     []Evaluation
	err        error
}

// reported spells what the engine claimed, `engine 'x' reports holds, bounded at depth 40`.
func (e externalEngine) reported(claim Claim, strength Strength, answer enginewire.Result) string {
	text := fmt.Sprintf("engine %q reports %s", e.Name(), claimSpelling(claim))
	if claim == ClaimNone {
		return text
	}
	text += ", " + strength.String()
	if reached := hostBounds(answer.Bounds); len(reached) > 0 {
		text += " at " + reached.String()
	}
	return text
}

// claimSpelling reads a claim as a reason names it, `a violation` for violated.
func claimSpelling(claim Claim) string {
	switch claim {
	case ClaimNone:
		return "no claim"
	case ClaimViolated:
		return "a violation"
	}
	return claim.String()
}

// hostBounds is the engine's bounds as the framework holds them.
func hostBounds(bounds []enginewire.Bound) Bounds {
	if len(bounds) == 0 {
		return nil
	}
	out := make(Bounds, len(bounds))
	for i, b := range bounds {
		out[i] = Bound{Name: b.Name, Limit: b.Limit, Reached: b.Reached}
	}
	return out
}

// hostValues reads the values the engine reports as the framework holds them, a unit read
// in the subject's library; a value the host cannot hold is kept as the evaluation's error.
func (e externalEngine) hostValues(model *Model, budget Budget, values []enginewire.Value) ([]Evaluation, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var ctx *runtime.Context
	if model.builds() {
		var err error
		if ctx, err = model.NewContext(budget); err != nil {
			return nil, err
		}
	}
	out := make([]Evaluation, len(values))
	for i, v := range values {
		out[i] = Evaluation{Name: v.Name}
		out[i].Value, out[i].Err = hostValue(ctx, v)
	}
	return out, nil
}

// hostValue is one value of the protocol as the run holds one, its unit read in the library.
func hostValue(ctx *runtime.Context, v enginewire.Value) (runtime.Value, error) {
	tool, err := readValue(v)
	if err != nil {
		return runtime.Value{}, &WireValueError{Name: v.Name, Held: err.Error()}
	}
	switch {
	case tool.Value.Kind == semantics.ValInvalid:
		return runtime.NewStringValue(tool.Text), nil
	case tool.Unit == "":
		return runtime.Value{Kind: runtime.ValConst, Const: tool.Value}, nil
	case ctx == nil:
		return runtime.Value{}, &WireValueError{Name: v.Name, Held: "a magnitude in " + tool.Unit + ", which no run reads a unit for"}
	}
	unit, err := ctx.UnitOf(nil, tool.Unit)
	if err != nil {
		return runtime.Value{}, &WireValueError{Name: v.Name, Held: err.Error()}
	}
	return runtime.NewQuantityValue(&runtime.Quantity{Num: tool.Value, Unit: unit}), nil
}

// ErrWitnessInputs is the typed refusal of a schedule witness carrying inputs.
var ErrWitnessInputs = errors.New("a schedule witness with inputs is not replayed")

// WitnessInputsError names the stage that replays a schedule with inputs.
type WitnessInputsError struct{ Engine string }

// Error names the engine and the stage.
func (e *WitnessInputsError) Error() string {
	return fmt.Sprintf("engine %q gives its schedule inputs, which are replayed with the free-inputs stage of the SMT engine", e.Engine)
}

// Is matches ErrWitnessInputs.
func (e *WitnessInputsError) Is(target error) bool { return target == ErrWitnessInputs }

// ErrNoReplay is the typed refusal of a witness on a question with no action to replay it on.
var ErrNoReplay = errors.New("no action to replay the witness on")

// NoReplayError names the question a schedule witness cannot be replayed for.
type NoReplayError struct {
	Engine string
	Kind   Kind
}

// Error names the engine and the question's kind.
func (e *NoReplayError) Error() string {
	return fmt.Sprintf("engine %q gives a schedule, and a %s question names no action to replay it on", e.Engine, e.Kind)
}

// Is matches ErrNoReplay.
func (e *NoReplayError) Is(target error) bool { return target == ErrNoReplay }

// replayable is the check ask a schedule witness replays through, or the typed refusal.
func (e externalEngine) replayable(q Question, w enginewire.Witness) (*CheckAsk, error) {
	if len(w.Inputs) > 0 {
		return nil, &WitnessInputsError{Engine: e.Name()}
	}
	if q.Check == nil || q.Check.Start == nil {
		return nil, &NoReplayError{Engine: e.Name(), Kind: q.Kind}
	}
	return q.Check, nil
}

// replayOne replays one schedule of the witness to the move at, on the plan's worker for
// job. A schedule that does not parse or replay is the standing's failure; the plan's
// clock ending is the error.
func (e externalEngine) replayOne(ctx context.Context, model *Model, budget Budget, ask *CheckAsk, job int, schedule string, at int) (*runtime.Replayed, *Witness, error, error) {
	choices, err := runtime.ParseChoices(schedule)
	if err != nil {
		return nil, nil, fmt.Errorf("its witness does not read as a schedule (%v)", err), nil
	}
	fresh := func() (*runtime.Context, error) { return model.NewContextOn(job, budget) }
	replayed, err := runtime.ReplaySchedule(ctx, fresh, ask.Start, choices, at)
	witness := &Witness{Schedule: runtime.ReplayPolicy(choices), Choices: choices}
	switch {
	case err == nil:
		return replayed, witness, nil, nil
	case ctx.Err() != nil:
		return nil, nil, nil, ctx.Err()
	case errors.Is(err, runtime.ErrReplayDisagrees):
		var disagreement *runtime.ReplayDisagreement
		reason := err.Error()
		if errors.As(err, &disagreement) {
			reason = disagreement.Reason
		}
		return nil, nil, fmt.Errorf("its witness does not replay (%s)", reason), nil
	}
	return nil, nil, nil, err
}

// holdsAt evaluates the question's claim on a replayed state: false with why when the run
// failed there or a property evaluates false.
func holdsAt(ask *CheckAsk, r *runtime.Replayed) (bool, string, error) {
	if r.Err != nil {
		return false, "the run fails there: " + r.Err.Error(), nil
	}
	for _, p := range ask.Properties {
		holds, err := r.Evaluate(p)
		if err != nil {
			return false, "", err
		}
		if !holds {
			return false, fmt.Sprintf("`%s` evaluates false there", p.Name), nil
		}
	}
	return true, "", nil
}

// standViolation replays the schedule to the move it names and asks the claim there: a
// violation stands when the run fails or a property evaluates false at that move.
func (e externalEngine) standViolation(ctx context.Context, model *Model, q Question, budget Budget, w enginewire.Witness) (standing, error) {
	ask, err := e.replayable(q, w)
	if err != nil {
		return standing{err: err}, nil
	}
	at := runtime.ScheduleEnd
	if w.At != nil {
		at = *w.At
	}
	replayed, witness, failed, err := e.replayOne(ctx, model, budget, ask, 0, w.Schedules[0], at)
	if err != nil || failed != nil {
		return standing{err: failed}, err
	}
	holds, why, err := holdsAt(ask, replayed)
	if err != nil {
		return standing{}, err
	}
	where := "at its end"
	if at != runtime.ScheduleEnd {
		where = fmt.Sprintf("at move %d", at)
	}
	if holds {
		return standing{err: fmt.Errorf("its schedule replays and every property holds %s", where)}, nil
	}
	return standing{strength: Witnessed, reason: fmt.Sprintf("%s (engine %q, replayed): %s", where, e.Name(), why), witness: witness}, nil
}

// standSensitivity replays both schedules to their end and compares the feature's final
// values: the sensitivity stands when they differ, the second schedule as the contrast.
func (e externalEngine) standSensitivity(ctx context.Context, model *Model, q Question, budget Budget, w enginewire.Witness) (standing, error) {
	ask, err := e.replayable(q, w)
	if err != nil {
		return standing{err: err}, nil
	}
	values := make([]string, 2)
	witnesses := make([]*Witness, 2)
	for i, schedule := range w.Schedules {
		replayed, witness, failed, err := e.replayOne(ctx, model, budget, ask, 0, schedule, runtime.ScheduleEnd)
		if err != nil {
			return standing{}, err
		}
		if failed != nil {
			return standing{err: fmt.Errorf("schedule %d: %v", i+1, failed)}, nil
		}
		value, err := replayed.FinalValue(w.Feature)
		if err != nil {
			if errors.Is(err, runtime.ErrUnknownCheckFeature) {
				return standing{err: fmt.Errorf("both schedules replay and `%s` is no feature of the run: %v", w.Feature, err)}, nil
			}
			return standing{}, err
		}
		values[i], witnesses[i] = value, witness
	}
	if values[0] == values[1] {
		return standing{err: fmt.Errorf("both schedules replay and `%s` is %s under each", w.Feature, values[0])}, nil
	}
	return standing{
		strength: Witnessed,
		reason:   fmt.Sprintf("%s ends as %s or %s (engine %q, both replayed)", w.Feature, values[0], values[1], e.Name()),
		witness:  witnesses[0], contrast: witnesses[1],
	}, nil
}

// standAssignment binds the witness's inputs to the queries' variables and confirms every
// query with the evaluator, as solve confirms a sat model; the assignment stands when all do.
func (e externalEngine) standAssignment(q Question, w enginewire.Witness) (standing, error) {
	if q.Solve == nil || len(q.Solve.Queries) == 0 {
		return standing{err: &MalformedQuestionError{Kind: q.Kind, Missing: "a Solve with its queries"}}, nil
	}
	given := make(map[string]runtime.ToolValue, len(w.Inputs))
	for _, in := range w.Inputs {
		value, err := readValue(in)
		if err != nil {
			return standing{err: fmt.Errorf("at its assignment %s is %v", in.Name, err)}, nil
		}
		if _, twice := given[in.Name]; twice {
			return standing{err: fmt.Errorf("its assignment gives %s twice", in.Name)}, nil
		}
		given[in.Name] = value
	}
	var values []Evaluation
	for _, query := range q.Solve.Queries {
		model := make(map[string]solve.ModelValue, len(query.Vars))
		assigned := make([]solve.Assignment, 0, len(query.Vars))
		for _, v := range query.Vars {
			tool, ok := given[v.Name]
			if !ok {
				return standing{err: fmt.Errorf("its assignment gives %s no value", v.Name)}, nil
			}
			value, err := v.ValueOf(tool)
			if err != nil {
				return standing{err: fmt.Errorf("at its assignment %s: %v", v.Name, err)}, nil
			}
			assignment, err := value.Assign(v)
			if err != nil {
				return standing{err: fmt.Errorf("at its assignment %s: %v", v.Name, err)}, nil
			}
			model[v.Name] = value
			assigned = append(assigned, assignment)
		}
		if ok, why := query.Confirm(model); !ok {
			return standing{err: fmt.Errorf("at its assignment %s", why)}, nil
		}
		values = append(values, Evaluation{Name: query.Element, Solved: &solve.Result{
			Query: query, Status: solve.StatusSat, Solver: "engine " + e.Name(), Model: assigned,
		}})
	}
	return standing{strength: Witnessed, reason: fmt.Sprintf("assignment of engine %q, confirmed by the evaluator", e.Name()), values: values}, nil
}

// standExecutions replays every execution a universal or concrete claim rests on and asks
// the claim at every settled state of each: the claim is observed on their count when all
// hold, not covered naming the first that does not, and not covered with the claim kept
// when there is nothing to replay.
func (e externalEngine) standExecutions(ctx context.Context, model *Model, q Question, budget Budget, claim Claim, answer enginewire.Result) (standing, error) {
	if len(answer.Executions) == 0 {
		if claim.Universal() {
			return standing{err: errors.New("not admitted")}, nil
		}
		return standing{err: errors.New("no execution to replay")}, nil
	}
	ask, err := e.replayable(q, answer.Executions[0])
	if err != nil {
		return standing{err: err}, nil
	}
	for _, x := range answer.Executions[1:] {
		if _, err := e.replayable(q, x); err != nil {
			return standing{err: err}, nil
		}
	}
	expected, err := e.hostValues(model, budget, answer.Values)
	if err != nil {
		return standing{}, err
	}
	jobs := max(budget.Jobs, 1)
	witnesses := make([]Witness, len(answer.Executions))
	failed := make([]error, len(answer.Executions))
	errs := make([]error, len(answer.Executions))
	var wg sync.WaitGroup
	for job := 0; job < jobs && job < len(answer.Executions); job++ {
		wg.Add(1)
		go func(job int) {
			defer wg.Done()
			for i := job; i < len(answer.Executions); i += jobs {
				witness, fail, err := e.replayExecution(ctx, model, budget, ask, job, claim, answer.Executions[i], expected)
				if witness != nil {
					witnesses[i] = *witness
				}
				failed[i], errs[i] = fail, err
			}
		}(job)
	}
	wg.Wait()
	for i := range answer.Executions {
		if errs[i] != nil {
			return standing{}, errs[i]
		}
		if failed[i] != nil {
			return standing{err: fmt.Errorf("execution %d: %v", i+1, failed[i])}, nil
		}
	}
	return standing{
		strength:   Observed,
		reason:     fmt.Sprintf("observed on %d executions chosen by engine %q, each replayed", len(witnesses), e.Name()),
		executions: witnesses,
	}, nil
}

// replayExecution replays one execution to its end, asking the claim at every settled
// state; a concrete claim is asked of the final values the engine reports.
func (e externalEngine) replayExecution(ctx context.Context, model *Model, budget Budget, ask *CheckAsk, job int, claim Claim, x enginewire.Witness, expected []Evaluation) (*Witness, error, error) {
	choices, err := runtime.ParseChoices(x.Schedules[0])
	if err != nil {
		return nil, fmt.Errorf("does not read as a schedule (%v)", err), nil
	}
	fresh := func() (*runtime.Context, error) { return model.NewContextOn(job, budget) }
	var fail error
	visit := func(r *runtime.Replayed, moves int) error {
		holds, why, err := holdsAt(ask, r)
		if err != nil {
			return err
		}
		if !holds {
			fail = fmt.Errorf("replays, and after %d moves %s", moves, why)
			return errVisitStopped
		}
		return nil
	}
	replayed, err := runtime.ReplayExecution(ctx, fresh, ask.Start, choices, visit)
	witness := &Witness{Schedule: runtime.ReplayPolicy(choices), Choices: choices}
	switch {
	case errors.Is(err, errVisitStopped):
		return witness, fail, nil
	case ctx.Err() != nil:
		return nil, nil, ctx.Err()
	case errors.Is(err, runtime.ErrReplayDisagrees):
		var disagreement *runtime.ReplayDisagreement
		reason := err.Error()
		if errors.As(err, &disagreement) {
			reason = disagreement.Reason
		}
		return witness, fmt.Errorf("does not replay (%s)", reason), nil
	case err != nil:
		return nil, nil, err
	}
	if replayed.Err != nil {
		return witness, fmt.Errorf("replays and fails at its end: %v", replayed.Err), nil
	}
	if claim.Concrete() {
		for _, v := range expected {
			if v.Err != nil {
				return witness, fmt.Errorf("reports %s as %v", v.Name, v.Err), nil
			}
			final, err := replayed.FinalValue(v.Name)
			if err != nil {
				if errors.Is(err, runtime.ErrUnknownCheckFeature) {
					return witness, fmt.Errorf("replays, and `%s` is no feature of the run: %v", v.Name, err), nil
				}
				return nil, nil, err
			}
			if want := runtime.FormatValue(v.Value); final != want {
				return witness, fmt.Errorf("replays and leaves %s as %s, not %s", v.Name, final, want), nil
			}
		}
	}
	return witness, nil, nil
}

// errVisitStopped ends a replay at the first settled state the claim does not hold on.
var errVisitStopped = errors.New("claim failed on the execution")
