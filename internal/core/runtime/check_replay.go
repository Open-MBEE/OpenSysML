package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// A witness re-runs through the scheduler seam: the `replay` policy follows its
// choices, and the trace the run leaves is compared with the witness's, line for
// line, until the two are one — the state the witness claims — or cannot be.

// ErrReplayDisagrees is the typed error every replay that reaches another state wraps.
var ErrReplayDisagrees = errors.New("replay disagrees with the witness")

// ReplayDisagreement reports a replay that left another trace than its witness:
// how, and the two traces.
type ReplayDisagreement struct {
	Reason  string
	Witness string
	Trace   string
}

func (e *ReplayDisagreement) Error() string {
	return fmt.Sprintf("%v: %s", ErrReplayDisagrees, e.Reason)
}

// Is makes every ReplayDisagreement match ErrReplayDisagrees.
func (e *ReplayDisagreement) Is(target error) bool { return target == ErrReplayDisagrees }

// Replayed is a witness re-run to the state it claims.
type Replayed struct {
	Ctx *Context
	Inv *Invocation
	// Err is the error the last move raised, nil when the state is one the run went on from.
	Err error
}

// Replay re-runs the witness: it starts the invocation start begins in the
// context fresh makes, under the `replay` policy over the witness (its inputs
// pinned, its choices followed), and moves it as the check did until its trace
// equals the witness's — or, for a witness ending in a failure, until a move
// raises it. A witness naming a
// property has that property, found among props, evaluated at the state reached:
// it must be false or fail as the witness says. The run failing to follow a
// choice, ending, failing otherwise or leaving another trace is a
// ReplayDisagreement; a caller that goes away mid-run takes the replay with it,
// its error being stop's.
func Replay(
	stop context.Context, fresh func() (*Context, error), start Starter, w Witness, props []CheckProperty,
) (*Replayed, error) {
	if err := stop.Err(); err != nil {
		return nil, err
	}
	var property *CheckProperty
	if w.Property != "" {
		at := slices.IndexFunc(props, func(p CheckProperty) bool { return p.Name == w.Property })
		if at < 0 {
			return nil, &ReplayDisagreement{Reason: "the witness names a property the replay was not given: " + w.Property, Witness: w.Trace}
		}
		property = &props[at]
	}
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	if err := ctx.SetSchedule(ReplayOf(w)); err != nil {
		return nil, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(NewTraceRecorder())
	}
	r := &Replayed{Ctx: ctx}
	run, err := beginInvocation(ctx, start)
	if err != nil {
		r.Err = err
		return r, r.agree(w, "starting the invocation failed")
	}
	if err := run.inv.started(ctx); err != nil {
		return nil, err
	}
	r.Inv = run.inv
	for {
		if err := stop.Err(); err != nil {
			return r, err
		}
		if err := run.stabilize(); err != nil {
			return r.failed(w, property, err)
		}
		if (w.Fails == "" || property != nil) && ctx.Trace().String() == w.Trace {
			if property != nil {
				return r, r.agreeOnProperty(w, *property)
			}
			return r, r.agree(w, "settling the claimed state")
		}
		if !strings.HasPrefix(w.Trace, ctx.Trace().String()) {
			return r, r.disagree(w, "the run left another trace")
		}
		if run.terminal() {
			return r, r.disagree(w, "the run ended before reaching the claimed state")
		}
		if err := run.step(owners(run.enabledMoves())); err != nil {
			return r.failed(w, property, err)
		}
	}
}

// failed ends the replay in the failure a move or the settling raised: the one
// the witness claims, or a disagreement.
func (r *Replayed) failed(w Witness, property *CheckProperty, err error) (*Replayed, error) {
	r.Err = err
	if property != nil {
		return r, r.disagree(w, "the run failed before reaching the claimed state: "+err.Error())
	}
	return r, r.agree(w, "the run failed: "+err.Error())
}

// agree checks the run followed its witness whole, left its trace and ended as
// it claims — in the failure it names, or in a state it goes on from — else
// disagrees for why.
func (r *Replayed) agree(w Witness, why string) error {
	if err := r.Ctx.Unfollowed(); err != nil {
		return r.disagree(w, err.Error())
	}
	if r.Ctx.Trace().String() != w.Trace {
		return r.disagree(w, why+" left another trace")
	}
	switch {
	case r.Err == nil && w.Fails != "":
		return r.disagree(w, "the run reached the claimed state without failing as claimed: "+w.Fails)
	case r.Err != nil && w.Fails == "":
		return r.disagree(w, "the run failed where the witness claims a state: "+r.Err.Error())
	case r.Err != nil && r.Err.Error() != w.Fails:
		return r.disagree(w, "the run failed otherwise than claimed: "+r.Err.Error()+", not "+w.Fails)
	}
	return nil
}

// agreeOnProperty checks the run reached the claimed state and the property
// there is false, or fails to evaluate, as the witness claims.
func (r *Replayed) agreeOnProperty(w Witness, p CheckProperty) error {
	if err := r.Ctx.Unfollowed(); err != nil {
		return r.disagree(w, err.Error())
	}
	if r.Ctx.Trace().String() != w.Trace {
		return r.disagree(w, "settling the claimed state left another trace")
	}
	if r.Err != nil {
		return r.disagree(w, "the run failed where the witness claims a state: "+r.Err.Error())
	}
	holds, err := r.evaluate(p)
	evaluating := "evaluating " + p.Name
	switch {
	case err != nil && w.Fails == "":
		return r.disagree(w, evaluating+" failed where the witness claims it false: "+err.Error())
	case err != nil && err.Error() != w.Fails:
		return r.disagree(w, evaluating+" failed otherwise than claimed: "+err.Error()+", not "+w.Fails)
	case err != nil:
		r.Err = err
	case w.Fails != "":
		return r.disagree(w, evaluating+" did not fail as claimed: "+w.Fails)
	case holds:
		return r.disagree(w, p.Name+" holds at the claimed state")
	}
	return nil
}

// evaluate asks the property of the replayed run under a readiness probe, as the check did.
func (r *Replayed) evaluate(p CheckProperty) (bool, error) {
	defer r.Ctx.beginProbe()()
	return p.Holds(r.Ctx, r.Inv)
}

func (r *Replayed) disagree(w Witness, reason string) error {
	return &ReplayDisagreement{Reason: reason, Witness: w.Trace, Trace: r.Ctx.Trace().String()}
}
