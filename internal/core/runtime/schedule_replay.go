package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ScheduleEnd asks ReplaySchedule for the run's end, every move followed.
const ScheduleEnd = -1

// ReplaySchedule re-runs the schedule the witness fixes, start beginning the invocation in
// the context fresh makes under the `replay` policy with the witness's inputs fixed first,
// to the stable state after move at (1-based) or, at ScheduleEnd, until the run ends with
// every choice followed. A run that cannot follow a choice or fix an input, ends or fails
// before the move, or ends with choices left is a ReplayDisagreement; a failure at the
// move named, or ending the run, is the Replayed's Err, for the caller to judge against
// the claim.
func ReplaySchedule(
	stop context.Context, fresh func() (*Context, error), start Starter, w Witness, at int,
) (*Replayed, error) {
	return replaySchedule(stop, fresh, start, w, at, nil)
}

// ReplayExecution re-runs the whole schedule as ReplaySchedule does at ScheduleEnd, calling
// visit at every stable state on the way, settled as a check settles, with the moves made so
// far. An error visit returns ends the replay with it.
func ReplayExecution(
	stop context.Context, fresh func() (*Context, error), start Starter, w Witness,
	visit func(r *Replayed, moves int) error,
) (*Replayed, error) {
	return replaySchedule(stop, fresh, start, w, ScheduleEnd, visit)
}

// replaySchedule is ReplaySchedule with an optional visitor of every settled state.
func replaySchedule(
	stop context.Context, fresh func() (*Context, error), start Starter, w Witness, at int,
	visit func(r *Replayed, moves int) error,
) (*Replayed, error) {
	if err := stop.Err(); err != nil {
		return nil, err
	}
	choices := w.Choices
	if at != ScheduleEnd && (at < 0 || at > len(choices)) {
		return nil, &ReplayDisagreement{
			Reason:  fmt.Sprintf("the witness names move %d of a schedule of %d moves", at, len(choices)),
			Witness: spellWitness(w),
		}
	}
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	if err := ctx.SetSchedule(ReplayOf(Witness{Objects: w.Objects, Inputs: w.Inputs, Choices: choices})); err != nil {
		return nil, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(NewTraceRecorder())
	}
	r := &Replayed{Ctx: ctx}
	run, err := beginInvocation(ctx, start)
	if err != nil {
		r.Err = err
		return r, r.disagreeOnSchedule(w, "starting the invocation failed: "+err.Error())
	}
	if err := run.inv.started(ctx); err != nil {
		return nil, err
	}
	r.Inv = run.inv
	for {
		if err := stop.Err(); err != nil {
			return r, err
		}
		taken := len(ctx.ChoicesTaken())
		if err := run.stabilize(); err != nil {
			return r, r.failedOnSchedule(w, err, taken, at)
		}
		if visit != nil {
			if err := visit(r, taken); err != nil {
				return r, err
			}
		}
		if at != ScheduleEnd && taken >= at {
			return r, nil
		}
		if run.terminal() {
			if err := ctx.Unfollowed(); err != nil {
				return r, r.disagreeOnSchedule(w, err.Error())
			}
			if at != ScheduleEnd {
				return r, r.disagreeOnSchedule(w, fmt.Sprintf("the run ended after %d moves, before move %d", taken, at))
			}
			return r, nil
		}
		if err := run.step(owners(run.enabledMoves())); err != nil {
			return r, r.failedOnSchedule(w, err, taken, at)
		}
	}
}

// failedOnSchedule ends a schedule replay in the failure a move or the settling raised:
// a refused choice or input, or a failure before the move named, disagrees; one at the
// end, the schedule followed whole, is the run's own.
func (r *Replayed) failedOnSchedule(w Witness, err error, taken, at int) error {
	r.Err = err
	switch {
	case errors.Is(err, ErrReplayRefused), errors.Is(err, ErrWitnessInput), errors.Is(err, ErrWitnessObject):
		return r.disagreeOnSchedule(w, err.Error())
	case at != ScheduleEnd:
		return r.disagreeOnSchedule(w, fmt.Sprintf("the run failed after %d moves, before move %d: %v", taken, at, err))
	}
	if left := r.Ctx.Unfollowed(); left != nil {
		return r.disagreeOnSchedule(w, left.Error())
	}
	return nil
}

// disagreeOnSchedule is a schedule replay's disagreement, the witness spelt without its trace.
func (r *Replayed) disagreeOnSchedule(w Witness, reason string) error {
	return &ReplayDisagreement{Reason: reason, Witness: spellWitness(w), Trace: r.Ctx.Trace().String()}
}

// spellWitness spells the witness's objects, inputs and schedule as its file holds them.
func spellWitness(w Witness) string {
	return Witness{Objects: w.Objects, Inputs: w.Inputs, Choices: w.Choices}.String()
}

// Evaluate asks the property of the replayed run's state under a readiness probe, as the check did.
func (r *Replayed) Evaluate(p CheckProperty) (bool, error) {
	return r.evaluate(p)
}

// Outcome spells the run's outcome as a check reports a final state's.
func (r *Replayed) Outcome() string {
	defer r.Ctx.beginProbe()()
	return r.Inv.Outcome().String()
}

// FinalValue spells the feature's value as the run left it, UnsetText for one holding none:
// `this.<name>` the performing object's, `<object>.<name>` one of several performing
// objects', a bare name the one behavior's own, `<behavior>.<name>` one of several's,
// `node.path` one under a node an action performed, `finalState` a machine's final
// configuration. A name nothing answers to is an UnknownCheckFeatureError.
func (r *Replayed) FinalValue(feature string) (string, error) {
	c := &checker{ctx: r.Ctx, inv: r.Inv, opts: CheckOptions{Diverge: []string{feature}}}
	if err := c.resolveDiverge(); err != nil {
		return "", err
	}
	c.tellHeld()
	if err := c.divergeReached(); err != nil {
		return "", err
	}
	values, _, _ := c.spellFinal()
	value, held := values[c.divergenceKey(feature)]
	if !held {
		return "", &UnknownCheckFeatureError{Name: feature, Reason: "the run left no value under it"}
	}
	return value, nil
}

// divergenceKey is the name divergence values hold the selected feature under: an
// action's own feature under the name its performance holds it by, any other as spelled.
func (c *checker) divergenceKey(feature string) string {
	if c.nested[feature] {
		return feature
	}
	prefixes := c.inv.prefixes()
	i, rest := c.inv.behaviorOf(prefixes, feature)
	if i < 0 || i >= len(c.inv.Actions) || strings.HasPrefix(feature, "this.") {
		return feature
	}
	return prefixes[i] + c.inv.Actions[i].root.key(rest)
}
