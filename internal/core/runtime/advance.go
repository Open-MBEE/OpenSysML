package runtime

import (
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// clockWaiter is an executor the shared clock drives: it registers its waits on
// the clock and is run when work of its is due at the current instant.
type clockWaiter interface {
	// dueLabel names the executor in a due-order choice; clockWaits lists its
	// waits on the clock, in due order.
	dueLabel() string
	clockWaits() []ClockWait
	// dueWork reports work runnable at the current instant; watchesChange a change
	// condition the executor polls once the definite work has settled.
	dueWork() bool
	watchesChange() bool
	// runDue runs the executor to quiescence at the current instant, counting
	// against progress, and reports whether it got anywhere.
	runDue(progress *dueProgress) (bool, error)
	// finished reports an executor the clock has nothing left to drive; running a
	// run of it already on the stack, which drives the clock itself.
	finished() bool
	running() bool
}

// dueProgress counts what one drive of the clock did, in the units the budgets
// bound: it is what a run's budget is measured against, over every executor.
type dueProgress struct {
	events, doSteps, steps int64
	// dropped are the signals dispatched that no transition consumed.
	dropped []Dispatch
	// settled are the executors a run at the current instant got nowhere with:
	// due again only once another gets somewhere or the clock moves.
	settled map[clockWaiter]bool
}

// settle records a run of w that got nowhere at this instant.
func (p *dueProgress) settle(w clockWaiter) {
	if p.settled == nil {
		p.settled = make(map[clockWaiter]bool)
	}
	p.settled[w] = true
}

// unsettle makes every executor due again: something got somewhere, or the clock moved.
func (p *dueProgress) unsettle() {
	clear(p.settled)
}

// noteDispatch records a dispatched signal that fired nothing.
func (p *dueProgress) noteDispatch(d Dispatch) {
	if _, isSignal := d.Event.Payload.(Message); isSignal && !d.Fired {
		p.dropped = append(p.dropped, d)
	}
}

// AdvanceReport is what one advance of the clock did.
type AdvanceReport struct {
	// From and To are the instants the clock moved between, in seconds.
	From, To float64
	// Events, DoSteps and Steps count the state events dispatched, the do
	// actions run and the action steps taken along the way.
	Events, DoSteps, Steps int64
	// Dropped are the signals dispatched along the way that no transition
	// consumed: deferred by the active state, or dropped.
	Dropped []Dispatch
	// Notes are what the advance noted: the choice points it drew and the
	// guards it could not evaluate, in order.
	Notes []RunNote
}

// Advance moves the clock by duration seconds, running everything due on the way
// instant by instant; a wait due later stays queued and nothing waiting is no error.
// The duration and the instant it leads to must be finite.
func (ctx *Context) Advance(duration float64) (AdvanceReport, error) {
	defer ctx.beginExecutorRun(&ctx.clockRun)()

	report := AdvanceReport{From: ctx.clock.now, To: ctx.clock.now}
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
		return report, fmt.Errorf("%w: cannot advance the clock by %s", ErrNegativeDuration, semantics.FormatReal(duration))
	}
	deadline, err := ctx.clock.instantAfter(duration, "an advance of")
	if err != nil {
		return report, err
	}
	noted := ctx.run.NoteCount()
	var progress dueProgress
	for {
		if _, err := ctx.runDue(nil, &progress); err != nil {
			report.To = ctx.clock.now
			return report.counting(progress, ctx.run.notes[noted:]), err
		}
		next, ok := ctx.clock.NextDue()
		if !ok || next > deadline {
			break
		}
		ctx.clock.now = next
		progress.unsettle()
	}
	ctx.clock.now = deadline
	report.To = deadline
	return report.counting(progress, ctx.run.notes[noted:]), nil
}

func (r AdvanceReport) counting(p dueProgress, notes []RunNote) AdvanceReport {
	r.Events, r.DoSteps, r.Steps, r.Dropped = p.events, p.doSteps, p.steps, p.dropped
	r.Notes = slices.Clone(notes)
	return r
}

// advanceToNextDue moves the clock to the earliest wait still ahead, whoever
// holds it, making every executor due again, and reports false when none is.
func (ctx *Context) advanceToNextDue(progress *dueProgress) bool {
	next, ok := ctx.clock.NextDue()
	if !ok {
		return false
	}
	ctx.clock.now = next
	progress.unsettle()
	return true
}

// runDue runs the executors due at the current instant until none is left — definite
// work first, then the change conditions watched, each round's order drawn by the
// scheduler — returning true instead of running the driver once the draw falls on it.
func (ctx *Context) runDue(driver clockWaiter, progress *dueProgress) (bool, error) {
	for {
		due := ctx.dueWaiters(driver, progress, clockWaiter.dueWork)
		if len(due) == 0 {
			due = ctx.dueWaiters(driver, progress, clockWaiter.watchesChange)
		}
		if len(due) == 0 {
			return false, nil
		}
		pick := 0
		if len(due) > 1 {
			pick = ctx.scheduling().pickDue(len(due))
			alternatives := make([]string, len(due))
			for i, w := range due {
				alternatives[i] = w.dueLabel()
			}
			ctx.noteChoice(ChoicePoint{
				Kind:         ChoiceDueOrder,
				Where:        "t=" + semantics.FormatReal(ctx.clock.now),
				Alternatives: alternatives,
				Taken:        pick,
			})
		}
		w := due[pick]
		if w == driver {
			return true, nil
		}
		moved, err := ctx.runWaiter(w, progress)
		if err != nil {
			return false, err
		}
		if moved {
			progress.unsettle()
		} else {
			progress.settle(w)
		}
	}
}

// runWaiter runs one executor's due work, recording the run when the executor
// is an object's behavior.
func (ctx *Context) runWaiter(w clockWaiter, progress *dueProgress) (bool, error) {
	if ctx.trace != nil {
		if behavior := ctx.behaviorOf(w); behavior != nil {
			ctx.trace.RecordBehaviorRun(behavior.Kind.String(), behavior.Name, behavior.Object.ID)
		}
	}
	moved, err := w.runDue(progress)
	if err != nil {
		if behavior := ctx.behaviorOf(w); behavior != nil {
			err = fmt.Errorf("%s: %w", behavior.Describe(), err)
		}
	}
	return moved, err
}

// dueWaiters lists the executors with work of the given kind due now, in creation
// order: those not already mid-run nor settled, and the driver whatever its run.
func (ctx *Context) dueWaiters(driver clockWaiter, progress *dueProgress, has func(clockWaiter) bool) []clockWaiter {
	ctx.clock.forgetFinished()
	var due []clockWaiter
	for _, w := range ctx.clock.waiters {
		if (w == driver || !w.running()) && !progress.settled[w] && has(w) {
			due = append(due, w)
		}
	}
	return due
}

// behaviorOf finds the object behavior an executor runs as, nil for one a
// caller drives directly.
func (ctx *Context) behaviorOf(w clockWaiter) *ObjectBehavior {
	for _, behavior := range ctx.objectBehaviors {
		if behavior.State != nil && clockWaiter(behavior.State) == w {
			return behavior
		}
		if behavior.Action != nil && clockWaiter(behavior.Action) == w {
			return behavior
		}
	}
	return nil
}
