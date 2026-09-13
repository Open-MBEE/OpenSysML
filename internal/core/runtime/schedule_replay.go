package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ScheduleEnd asks ReplaySchedule for the run's end, every move followed.
const ScheduleEnd = -1

// ReplaySchedule re-runs the schedule the choices fix, start beginning the action in the
// context fresh makes under the `replay` policy, to the stable state after move at
// (1-based) or, at ScheduleEnd, until the run completes with every choice followed. A run
// that cannot follow a choice, completes or fails before the move, or ends with choices left
// is a ReplayDisagreement; a failure at the move named, or ending the run, is the Replayed's
// Err, for the caller to judge against the claim.
func ReplaySchedule(
	stop context.Context, fresh func() (*Context, error), start ActionStarter, choices []ChoiceTaken, at int,
) (*Replayed, error) {
	if err := stop.Err(); err != nil {
		return nil, err
	}
	if at != ScheduleEnd && (at < 0 || at > len(choices)) {
		return nil, &ReplayDisagreement{
			Reason:  fmt.Sprintf("the witness names move %d of a schedule of %d moves", at, len(choices)),
			Witness: spellChoices(choices),
		}
	}
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	if err := ctx.SetSchedule(ReplayPolicy(choices)); err != nil {
		return nil, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(NewTraceRecorder())
	}
	r := &Replayed{Ctx: ctx}
	exec, err := start(ctx)
	if err != nil {
		r.Err = err
		return r, r.disagreeOnSchedule(choices, "starting the action failed: "+err.Error())
	}
	r.Exec = exec
	for {
		if err := stop.Err(); err != nil {
			return r, err
		}
		taken := len(ctx.ChoicesTaken())
		if at != ScheduleEnd && taken >= at {
			if err := r.settle(stop); err != nil {
				if stop.Err() != nil {
					return r, err
				}
				r.Err = err
			}
			return r, nil
		}
		if exec.State() == StateCompleted {
			if at != ScheduleEnd {
				return r, r.disagreeOnSchedule(choices, fmt.Sprintf("the run completed after %d moves, before move %d", taken, at))
			}
			if err := ctx.Unfollowed(); err != nil {
				return r, r.disagreeOnSchedule(choices, err.Error())
			}
			return r, nil
		}
		before := len(ctx.Trace().Entries())
		now := ctx.clock.now
		if err := exec.advance(); err != nil {
			r.Err = err
			switch {
			case errors.Is(err, ErrReplayRefused):
				return r, r.disagreeOnSchedule(choices, err.Error())
			case at != ScheduleEnd:
				return r, r.disagreeOnSchedule(choices, fmt.Sprintf("the run failed after %d moves, before move %d: %v", taken, at, err))
			}
			if left := ctx.Unfollowed(); left != nil {
				return r, r.disagreeOnSchedule(choices, left.Error())
			}
			return r, nil
		}
		if len(ctx.Trace().Entries()) == before && ctx.clock.now == now && exec.State() == StateWaiting {
			return r, r.disagreeOnSchedule(choices, fmt.Sprintf("the run is stuck after %d moves", taken))
		}
	}
}

// disagreeOnSchedule is a schedule replay's disagreement, the schedule standing for the witness.
func (r *Replayed) disagreeOnSchedule(choices []ChoiceTaken, reason string) error {
	return &ReplayDisagreement{Reason: reason, Witness: spellChoices(choices), Trace: r.Ctx.Trace().String()}
}

// spellChoices spells the schedule as its witness file holds it.
func spellChoices(choices []ChoiceTaken) string {
	return Witness{Choices: choices}.String()
}

// Evaluate asks the property of the replayed run's state under a readiness probe, as the check did.
func (r *Replayed) Evaluate(p CheckProperty) (bool, error) {
	return r.evaluate(p)
}

// Outcome spells the run's outcome as a check reports a final state's.
func (r *Replayed) Outcome() string {
	defer r.Ctx.beginProbe()()
	return r.Ctx.ActionOutcome(r.Exec.Results()).String()
}

// FinalValue spells the feature's value as the run left it, UnsetText for one holding none:
// `this.<name>` the performing object's, a bare name the action's own, `node.path` one
// under a node it performed. A name nothing answers to is an UnknownCheckFeatureError.
func (r *Replayed) FinalValue(feature string) (string, error) {
	c := &checker{ctx: r.Ctx, exec: r.Exec, opts: CheckOptions{Diverge: []string{feature}}}
	if err := c.resolveDiverge(); err != nil {
		return "", err
	}
	c.tellHeld()
	if err := c.divergeReached(); err != nil {
		return "", err
	}
	values, _, _ := c.spellFinal()
	key := feature
	if !strings.HasPrefix(feature, "this.") && !c.nested[feature] {
		key = r.Exec.root.key(feature)
	}
	value, held := values[key]
	if !held {
		return "", &UnknownCheckFeatureError{Name: feature, Reason: "the run left no value under it"}
	}
	return value, nil
}
