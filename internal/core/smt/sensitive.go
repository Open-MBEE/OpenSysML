package smt

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// NoSensitivityWithin begins the reason of a bounded negative sensitivity answer: no
// two completed schedules differ, but some schedule is cut by the bounds.
const NoSensitivityWithin = "no sensitivity found within %d moves"

// Refusal reasons for the features the two-copy query does not range over.
const (
	performerReason = "the performing object's features are encoded by a later stage"
	nestedReason    = "the features a node's performance holds are compared by the check engine only"
)

// featureRefusal is a named feature the two-copy query does not range over, and why.
type featureRefusal struct {
	feature string
	reason  string
}

// compared is the feature list a Sensitive question compares: the action's outputs
// the two-copy query ranges over, and the features it refuses by name.
type compared struct {
	outputs []Output
	refused []featureRefusal
}

// features resolves the question's Diverge list, as encode had the interpreter tell it:
// the action's own features are compared, the performing object's and those under a
// node's performance are refused by name. Absent names, every one of the action's own
// and of the performing object's attributes is compared.
func (r *run) features() *compared {
	outputs := r.encoding.Outputs()
	c := &compared{}
	if len(r.q.Holds.Diverge) == 0 {
		c.outputs = outputs
		for _, name := range r.performer {
			c.refused = append(c.refused, featureRefusal{feature: "this." + name, reason: performerReason})
		}
		return c
	}
	for _, name := range r.q.Holds.Diverge {
		switch r.compared[name] {
		case runtime.OwnedByPerformer:
			c.refused = append(c.refused, featureRefusal{feature: name, reason: performerReason})
		case runtime.OwnedByNode:
			c.refused = append(c.refused, featureRefusal{feature: name, reason: nestedReason})
		default:
			if at := slices.IndexFunc(outputs, func(o Output) bool { return o.Name == name }); at >= 0 {
				c.outputs = append(c.outputs, outputs[at])
			}
		}
	}
	return c
}

// decideSensitive answers a Sensitive question in the order the design spells: a
// violation of the conditions asked with it first; then, per feature, the two-copy
// query, a `sat` replayed twice before it is claimed; then the deadlock and typed-error
// properties, whose `sat` is that finding; then whether any schedule is cut by the
// bounds, which tells a bounded negative from a proved one. An undecided or refused
// feature does not stop the list, but leaves a negative over it not covered.
func (r *run) decideSensitive(ctx context.Context) (analysis.Result, error) {
	if out, err := r.consistent(ctx); out != nil || err != nil {
		return orEmpty(out), err
	}
	features := r.features()
	if len(features.outputs) == 0 {
		out := r.uncovered(refusalReason(features.refused))
		out.Values = r.refusals(features)
		return out, nil
	}
	asks := r.asks()
	var before []ask
	if r.property != r.deadlock {
		before, asks = asks[:1], asks[1:]
	}
	found, rounded, err := r.findings(ctx, before)
	if found != nil || err != nil {
		return orEmpty(found), err
	}
	solved := make([]analysis.Evaluation, 0, len(features.outputs)+len(features.refused))
	var undecided *analysis.Result
	for _, out := range features.outputs {
		pair, err := r.diverging(ctx, out, &rounded, &undecided)
		if err != nil {
			return analysis.Result{}, err
		}
		solved = append(solved, analysis.Evaluation{Name: out.Name, Solved: pair.result})
		if pair.Diverging != nil {
			return r.sensitive(ctx, pair.Diverging, append(solved, r.refusals(features)...))
		}
	}
	solved = append(solved, r.refusals(features)...)
	found, roundedAfter, err := r.findings(ctx, asks)
	if found != nil || err != nil {
		return orEmpty(found), err
	}
	if undecided != nil {
		undecided.Values = solved
		return *undecided, nil
	}
	if len(features.refused) > 0 {
		out := r.uncovered(refusalReason(features.refused))
		out.Values = solved
		return out, nil
	}
	if rounded == nil {
		rounded = roundedAfter
	}
	if rounded != nil {
		return *rounded, nil
	}
	return r.completes(ctx, func(cut Cut, result *solve.Result) (analysis.Result, error) {
		live, err := r.encoding.LiveSchedule(result)
		if err != nil {
			return analysis.Result{}, err
		}
		out := r.holds(analysis.Bounded, cut)
		out.Reason = fmt.Sprintf(NoSensitivityWithin, r.moves) + ": " + r.cutReason(cut, live)
		out.Values = solved
		return out, nil
	}, func() analysis.Result {
		out := r.holds(analysis.Proved, Cut{})
		out.Values = solved
		return out
	})
}

// asked is the solver's answer to one feature's two-copy query, decoded on `sat`.
type asked struct {
	result *solve.Result
	// Diverging is nil for an `unsat` and for an answer that decides nothing.
	*Diverging
}

// diverging asks the two-copy query over out: the answer with its pair decoded on
// `sat`. The first `unsat` that rounds is noted in rounded; the first answer deciding
// nothing — the solver did not decide, or the model does not decode — in undecided.
func (r *run) diverging(ctx context.Context, out Output, rounded, undecided **analysis.Result) (*asked, error) {
	what := "whether two schedules end with different values of " + out.Name
	query, err := r.encoding.Sensitivity(out)
	if err != nil {
		return nil, err
	}
	result, err := r.solver.Solve(ctx, query)
	if err != nil {
		return nil, err
	}
	note := func(answer analysis.Result) {
		if *undecided == nil {
			*undecided = &answer
		}
	}
	switch result.Status {
	case solve.StatusUnsat:
		if *rounded == nil && query.Rounded() {
			answer := r.rounded(result, what)
			*rounded = &answer
		}
		return &asked{result: result}, nil
	case solve.StatusSat:
		pair, err := r.encoding.DecodePair(result, out)
		if err != nil {
			var malformed *WitnessError
			if errors.As(err, &malformed) || errors.Is(err, ErrNoWitness) {
				note(r.uncovered("the solver's witness does not decode: " + err.Error()))
				return &asked{result: result}, nil
			}
			return nil, err
		}
		return &asked{result: result, Diverging: pair}, nil
	default:
		note(r.undecided(result, what))
		return &asked{result: result}, nil
	}
}

// refusalReason spells why each refused feature is, one clause per feature.
func refusalReason(refused []featureRefusal) string {
	parts := make([]string, len(refused))
	for i, f := range refused {
		parts[i] = f.feature + ": " + f.reason
	}
	return strings.Join(parts, "; ")
}

// refusals is the refused features as evaluations, each with its typed refusal.
func (r *run) refusals(features *compared) []analysis.Evaluation {
	values := make([]analysis.Evaluation, 0, len(features.refused))
	for _, f := range features.refused {
		values = append(values, analysis.Evaluation{Name: f.feature, Err: &analysis.ConstructError{Engine: r.engine.Name(), Construct: f.reason}})
	}
	return values
}

// cutReason spells what keeps a schedule from completing within the bounds: the
// schedule still live at the last move, the loop or the fork the bounds cut.
func (r *run) cutReason(cut Cut, live *Witness) string {
	var parts []string
	if cut.Moves {
		schedule := "no choice points"
		if live != nil && len(live.Choices) > 0 {
			schedule = runtime.FormatChoices(live.Choices)
		}
		parts = append(parts, fmt.Sprintf("a schedule is still live after move %d (%s)", r.moves, schedule))
	}
	if cut.Unroll {
		parts = append(parts, fmt.Sprintf("a body loop runs past its unrolling of %d", r.encoding.Unroll))
	}
	if cut.Slots {
		parts = append(parts, fmt.Sprintf("a fork finds no free token slot of %d", r.encoding.Flow.Slots))
	}
	return strings.Join(parts, "; ")
}

// sensitive replays the two runs of a diverging pair, each to its end, and claims the
// sensitivity only when the interpreter leaves the feature at the value each copy
// says; a run that does not is the disagreement, in the interpreter's favor.
func (r *run) sensitive(ctx context.Context, pair *Diverging, solved []analysis.Evaluation) (analysis.Result, error) {
	var witnesses [2]*analysis.Witness
	for i, w := range pair.Runs {
		witness, disagreement, err := r.replayRun(ctx, w, pair.Feature, pair.Values[i], CopyNames[i])
		if err != nil {
			return analysis.Result{}, err
		}
		witnesses[i] = witness
		if disagreement != "" {
			out := r.uncovered(disagreement)
			out.Witness, out.Contrast = witnesses[0], witnesses[1]
			out.Inputs = r.inputs(w.Inputs)
			return out, nil
		}
	}
	out := r.result()
	out.Claim, out.Strength = analysis.ClaimSensitive, analysis.Witnessed
	out.Witness, out.Contrast = witnesses[0], witnesses[1]
	out.Inputs = r.inputs(pair.Runs[0].Inputs)
	out.Reason = pair.String()
	out.Values = solved
	return out, nil
}

// replayRun re-runs one schedule of a pair through the interpreter to its end and reads
// the feature's final value; the witness is then written, with the trace the run left,
// when the question names a directory, and replayed from what was written as a violation
// witness is. It returns the disagreement, "" when the interpreter agrees.
func (r *run) replayRun(ctx context.Context, w *Witness, feature, expected, copy string) (*analysis.Witness, string, error) {
	fresh := func() (*runtime.Context, error) { return r.model.NewContextOn(0, r.budget) }
	start := runtime.ActionStarter(r.q.Holds.Start)
	witness := &analysis.Witness{Schedule: w.policy(), Inputs: w.Inputs, Choices: w.Choices}
	claim := fmt.Sprintf("the solver claims %s ends as %s under schedule %s", feature, expected, copy)
	file := runtime.Witness{Inputs: w.Inputs, Choices: w.Choices}
	replayed, err := runtime.ReplaySchedule(ctx, fresh, start, file, runtime.ScheduleEnd)
	switch {
	case errors.Is(err, runtime.ErrReplayDisagrees):
		return witness, claim + "; " + err.Error(), nil
	case err != nil:
		return nil, "", err
	case replayed.Err != nil:
		return witness, fmt.Sprintf("%s; the interpreter's run fails: %v", claim, replayed.Err), nil
	case replayed.Exec.State() != runtime.StateCompleted:
		return witness, claim + "; the interpreter's run does not complete", nil
	}
	value, err := replayed.FinalValue(feature)
	switch {
	case errors.Is(err, runtime.ErrUnknownCheckFeature):
		return witness, fmt.Sprintf("%s; %v", claim, err), nil
	case err != nil:
		return nil, "", err
	case value != expected:
		return witness, fmt.Sprintf("%s; the interpreter leaves it as %s", claim, value), nil
	}
	file.Trace = replayed.Ctx.Trace().String()
	if r.q.Holds.WitnessDir != "" {
		path := filepath.Join(r.q.Holds.WitnessDir, analysis.SensitivityFile(r.q.Subject, r.q.Holds.Performer, feature, copy))
		if err := analysis.WriteWitness(path, file.String()); err != nil {
			return nil, "", err
		}
		witness.Written = path
	}
	if _, err := runtime.ReplayAction(ctx, fresh, start, file, nil); err != nil {
		if errors.Is(err, runtime.ErrReplayDisagrees) {
			return witness, claim + "; " + err.Error(), nil
		}
		return nil, "", err
	}
	return witness, "", nil
}
