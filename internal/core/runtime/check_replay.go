package runtime

import (
	"context"
	"errors"
	"fmt"
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
	Ctx  *Context
	Exec *ActionExecutor
	// Err is the error the last move raised, nil when the state is one the run went on from.
	Err error
}

// ReplayAction re-runs the witness: it starts the action start begins in the
// context fresh makes, under the `replay` policy over the witness's choices, and
// steps it until its trace equals the witness's, settling as the check did. The
// run failing to follow a choice, ending or leaving another trace is a
// ReplayDisagreement; a caller that goes away mid-run takes the replay with it,
// its error being stop's.
func ReplayAction(stop context.Context, fresh func() (*Context, error), start ActionStarter, w Witness) (*Replayed, error) {
	if err := stop.Err(); err != nil {
		return nil, err
	}
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	if err := ctx.SetSchedule(ReplayPolicy(w.Choices)); err != nil {
		return nil, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(NewTraceRecorder())
	}
	r := &Replayed{Ctx: ctx}
	exec, err := start(ctx)
	if err != nil {
		r.Err = err
		return r, r.agree(w, "starting the action failed")
	}
	r.Exec = exec
	for {
		if err := stop.Err(); err != nil {
			return r, err
		}
		if ctx.Trace().String() == w.Trace {
			if err := r.settle(stop); err != nil {
				if stop.Err() != nil {
					return r, err
				}
				r.Err = err
			}
			return r, r.agree(w, "settling the claimed state")
		}
		if !strings.HasPrefix(w.Trace, ctx.Trace().String()) {
			return r, r.disagree(w, "the run left another trace")
		}
		if exec.State() == StateCompleted {
			return r, r.disagree(w, "the run completed before reaching the claimed state")
		}
		before := len(ctx.Trace().Entries())
		if err := exec.advance(); err != nil {
			r.Err = err
			return r, r.agree(w, "the run failed: "+err.Error())
		}
		if len(ctx.Trace().Entries()) == before && exec.State() == StateWaiting {
			return r, r.disagree(w, "the run is stuck before reaching the claimed state")
		}
	}
}

// settle brings the replayed run to the stable state the check evaluated: complete, or with a move enabled.
func (r *Replayed) settle(stop context.Context) error {
	for r.Exec.State() != StateCompleted && len(r.Exec.enabledMoves()) == 0 {
		if err := stop.Err(); err != nil {
			return err
		}
		before := len(r.Ctx.Trace().Entries())
		now := r.Ctx.clock.now
		if err := r.Exec.advance(); err != nil {
			return err
		}
		if len(r.Ctx.Trace().Entries()) == before && r.Ctx.clock.now == now {
			return r.Exec.deadlockError(nil)
		}
	}
	return nil
}

// agree checks the run followed its witness whole and left its trace, else disagrees for why.
func (r *Replayed) agree(w Witness, why string) error {
	if err := r.Ctx.Unfollowed(); err != nil {
		return r.disagree(w, err.Error())
	}
	if r.Ctx.Trace().String() != w.Trace {
		return r.disagree(w, why+" left another trace")
	}
	return nil
}

func (r *Replayed) disagree(w Witness, reason string) error {
	return &ReplayDisagreement{Reason: reason, Witness: w.Trace, Trace: r.Ctx.Trace().String()}
}

// advance makes one step of the run under whatever policy it is under, with the
// clock moved as a check moves it: to the earliest wait when nothing else can act.
func (e *ActionExecutor) advance() error {
	err := e.Step()
	switch {
	case errors.Is(err, ErrNothingDue):
		return e.advanceClock()
	case err == nil && e.state == StateWaiting && e.waitsOnClock(nil):
		return e.advanceClock()
	case err == nil && e.state == StateWaiting:
		return e.deadlockError(nil)
	}
	return err
}

// String renders the witness as a file holds it: its choices one per line — or
// `no choice points` — a blank line, and the trace.
func (w Witness) String() string {
	var b strings.Builder
	if len(w.Choices) == 0 {
		b.WriteString("no choice points\n")
	}
	for _, c := range w.Choices {
		b.WriteString(c.String())
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(w.Trace)
	return b.String()
}

// ParseWitness reads a witness as Witness.String writes it: the choices
// ParseChoices reads, and after the blank line ending them the trace, exact.
func ParseWitness(text string) (Witness, error) {
	choices, err := ParseChoices(text)
	if err != nil {
		return Witness{}, err
	}
	lines := strings.SplitAfter(text, "\n")
	begun := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			if begun {
				return Witness{Choices: choices, Trace: strings.Join(lines[i+1:], "")}, nil
			}
			continue
		}
		begun = true
	}
	return Witness{Choices: choices}, nil
}
