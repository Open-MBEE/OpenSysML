package runtime

import (
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// clockWaiter is an executor the shared clock drives: it registers its waits on
// the clock and is run when work of its is due at the current instant.
type clockWaiter interface {
	// dueLabel names the executor in a due-order choice; clockWaits lists its waits on
	// the clock not yet due, armedWaits every one, due or not, in due order, and
	// visibleArmedWaits those and the waits of the executors its paused work performs.
	dueLabel() string
	clockWaits() []ClockWait
	armedWaits() []ClockWait
	visibleArmedWaits() []ClockWait
	// dueWork reports work runnable at the current instant; watchesChange a change
	// condition the executor polls once the definite work has settled.
	dueWork() bool
	watchesChange() bool
	// runDue runs the executor to quiescence at the current instant, counting
	// against progress, and reports whether it got anywhere.
	runDue(progress *dueProgress) (bool, error)
	// runMove runs one move of the executor's due work, as runDue does.
	runMove(progress *dueProgress) (bool, error)
	// finished reports an executor the clock has nothing left to drive; running a
	// run of it already on the stack, which drives the clock itself.
	finished() bool
	running() bool
	// Release withdraws the executor from the clock for good.
	Release()
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

// noteDispatch records a dispatched signal nothing took: no transition fired on
// it and no do behavior went on with it. The do behaviors resumed count as do steps.
func (p *dueProgress) noteDispatch(d Dispatch) {
	p.doSteps += int64(len(d.Resumed))
	if _, isSignal := d.Event.Payload.(Message); isSignal && !d.Fired && len(d.Resumed) == 0 {
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
	// consumed and no do behavior went on with: deferred by the active state, or dropped.
	Dropped []Dispatch
	// Notes are what the advance noted: the choice points it drew and the
	// guards it could not evaluate, in order.
	Notes []RunNote
}

// Advance moves the clock by duration seconds, running everything due on the way
// instant by instant; a wait due later stays queued and nothing waiting is no error.
// The duration and the instant it leads to must be finite.
func (ctx *Context) Advance(duration float64) (AdvanceReport, error) {
	return ctx.AdvanceUntil(duration, nil)
}

// AdvanceUntil is Advance stopping early, the clock held at the instant whose
// due work made halted true — a debugger's breakpoint — before moving on.
func (ctx *Context) AdvanceUntil(duration float64, halted func() bool) (AdvanceReport, error) {
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
		// One executor at a time, so a halt is seen before the next due one runs.
		for {
			ran, _, err := ctx.stepDue(nil, &progress)
			if err != nil {
				report.To = ctx.clock.now
				return report.counting(progress, ctx.run.notes[noted:]), err
			}
			if halted != nil && halted() {
				report.To = ctx.clock.now
				return report.counting(progress, ctx.run.notes[noted:]), nil
			}
			if !ran {
				break
			}
		}
		next, ok := ctx.clock.NextDue()
		if !ok || next > deadline {
			break
		}
		ctx.setClock(next)
		progress.unsettle()
	}
	ctx.setClock(deadline)
	report.To = deadline
	return report.counting(progress, ctx.run.notes[noted:]), ctx.advanceEnded()
}

// advanceEnded is the refusal of an advance of its own that finished every executor
// on the clock with witness moves left over; nil while one may still move.
func (ctx *Context) advanceEnded() error {
	for _, w := range ctx.clock.waiters {
		if !w.finished() {
			return nil
		}
	}
	return ctx.endedWhole(&ctx.clockRun)
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
	ctx.setClock(next)
	progress.unsettle()
	return true
}

// runDue runs the executors due at the current instant until none is left — definite
// work first, then the change conditions watched, each round's order drawn by the
// scheduler — returning true instead of running the driver once the draw falls on it.
func (ctx *Context) runDue(driver clockWaiter, progress *dueProgress) (bool, error) {
	for {
		ran, yours, err := ctx.stepDue(driver, progress)
		if err != nil || !ran {
			return false, err
		}
		if yours {
			return true, nil
		}
	}
}

// stepDue draws one of the executors due at the current instant and runs it, unless
// it is the driver, which is left for the driver itself; ran is false when nothing
// is due.
func (ctx *Context) stepDue(driver clockWaiter, progress *dueProgress) (ran, yours bool, err error) {
	due := ctx.dueWaiters(driver, progress, clockWaiter.dueWork)
	if len(due) == 0 {
		due = ctx.dueWaiters(driver, progress, clockWaiter.watchesChange)
	}
	if len(due) == 0 {
		return false, false, nil
	}
	pick, err := ctx.drawDueOrder(due)
	if err != nil {
		return false, false, err
	}
	w := due[pick]
	if w == driver {
		return true, true, nil
	}
	moved, err := ctx.runWaiter(w, progress)
	if err != nil {
		return false, false, err
	}
	if moved {
		progress.unsettle()
	} else {
		progress.settle(w)
	}
	return true, false, nil
}

// drawDueOrder resolves which of the executors due runs first: the only one where
// there is one, else the policy's pick, noted as a due-order choice.
func (ctx *Context) drawDueOrder(due []clockWaiter) (int, error) {
	if len(due) < 2 {
		return 0, nil
	}
	choice := ChoicePoint{
		Kind:         ChoiceDueOrder,
		Where:        "t=" + semantics.FormatReal(ctx.clock.now),
		Alternatives: make([]string, len(due)),
	}
	for i, w := range due {
		choice.Alternatives[i] = w.dueLabel()
	}
	scheduling := ctx.scheduling()
	pick := scheduling.choose(choice, nil)
	if err := scheduling.refusal(); err != nil {
		return 0, err
	}
	choice.Taken = pick
	ctx.noteChoice(choice)
	return pick, nil
}

// contended reports whether a move w may make next depends on a move another
// executor may make before it at this instant, under a policy interleaving turns.
func (ctx *Context) contended(w clockWaiter) bool {
	if !ctx.gateFutures() {
		return false
	}
	next := nextFootprint(w)
	for _, other := range ctx.rivals(w) {
		if next.DependentBy(ctx.futures.executorFuture(other), ctx.relation(w, other)) {
			return true
		}
	}
	return false
}

// gateFutures readies the futures of this instant's executors under a policy
// interleaving turns, reporting whether one does.
func (ctx *Context) gateFutures() bool {
	if sched := ctx.scheduling(); !sched.interleavesTurns() && !sched.checking() {
		return false
	}
	if ctx.futures == nil {
		ctx.futures = newFutureFootprints(ctx)
	}
	ctx.futures.gate()
	return true
}

// endContended reports whether the driver's next move may end its run while
// another executor may still move at this instant: an end observes every place.
func (ctx *Context) endContended(driver clockWaiter) bool {
	return mayEnd(driver) && ctx.gateFutures() && len(ctx.rivals(driver)) > 0
}

// mayEnd reports whether the executor's next move may end its performance: a
// token of an action that may, or a machine that can complete or terminate.
func mayEnd(w clockWaiter) bool {
	switch exec := w.(type) {
	case *ActionExecutor:
		return exec.dynamics != nil || slices.ContainsFunc(exec.tokens, func(t Token) bool { return tokenMayEnd(exec, t) })
	case *StateExecutor:
		return len(exec.graph.TopRegions) > 0 || len(exec.graph.Terminates) > 0 || slices.ContainsFunc(exec.graph.States, exec.graph.Completes)
	}
	return true
}

// tokenMayEnd reports whether the token's next move may end the performance: a
// move out of a nested frame, past no unguarded succession, or reaching beyond its node.
func tokenMayEnd(exec *ActionExecutor, t Token) bool {
	if t.frame != nil && t.frame != exec.root || standing(exec, t).Dynamic {
		return true
	}
	edges := tokenGraphOf(exec, t).Edges[t.Location]
	return len(edges) == 0 || slices.ContainsFunc(edges, func(e lower.ActionEdge) bool { return e.Guard != nil })
}

// rivals are the executors other than w that may move at this instant before w's
// next move: those due or on the stack, and those a rival's moves may wake.
func (ctx *Context) rivals(w clockWaiter) []clockWaiter {
	var rivals, idle []clockWaiter
	for _, other := range ctx.clock.waiters {
		switch {
		case other == w || other.finished():
		case other.dueWork() || other.running():
			rivals = append(rivals, other)
		default:
			idle = append(idle, other)
		}
	}
	for woke := true; woke; {
		woke = false
		for i, other := range idle {
			if other != nil && ctx.wokenBy(other, rivals) {
				rivals = append(rivals, other)
				idle[i], woke = nil, true
			}
		}
	}
	return rivals
}

// wokenBy reports whether a rival's future may not commute with the idle executor's.
func (ctx *Context) wokenBy(idle clockWaiter, rivals []clockWaiter) bool {
	future := ctx.futures.executorFuture(idle)
	for _, rival := range rivals {
		if ctx.futures.executorFuture(rival).DependentBy(future, ctx.relation(rival, idle)) {
			return true
		}
	}
	return false
}

// nextFootprint is what any move the executor may make next touches.
func nextFootprint(w clockWaiter) lower.Footprint {
	switch exec := w.(type) {
	case *ActionExecutor:
		var fp lower.Footprint
		for _, t := range exec.tokens {
			fp = unionFootprints(fp, standing(exec, t))
		}
		return fp
	case *StateExecutor:
		return machineStanding(exec)
	}
	return lower.Footprint{Dynamic: true}
}

// runTurn runs the executor's due work, holding its turn only through moves
// independent of every other executor's, so the due order is drawn before each other.
func (ctx *Context) runTurn(w clockWaiter, progress *dueProgress) (bool, error) {
	if !ctx.scheduling().interleavesTurns() {
		return w.runDue(progress)
	}
	ran := false
	for {
		moved, err := w.runMove(progress)
		ran = ran || moved
		if err != nil || !moved || !w.dueWork() || ctx.contended(w) {
			return ran, err
		}
	}
}

// yieldTurn draws the due order again before a driver's move contended by another
// executor, running the executors the draw falls on before the driver goes on.
func (ctx *Context) yieldTurn(driver clockWaiter, progress *dueProgress) error {
	if ctx.body != nil || !ctx.contended(driver) && !ctx.endContended(driver) {
		return nil
	}
	for _, w := range ctx.clock.waiters {
		if w != driver && w.running() {
			return nil
		}
	}
	_, err := ctx.runDue(driver, progress)
	return err
}

// runWaiter runs one executor's turn of due work, recording the run when the
// executor is an object's behavior.
func (ctx *Context) runWaiter(w clockWaiter, progress *dueProgress) (bool, error) {
	if ctx.trace != nil {
		if behavior := ctx.behaviorOf(w); behavior != nil {
			ctx.trace.RecordBehaviorRun(behavior.Kind.String(), behavior.Name, behavior.Object.ID)
		}
	}
	moved, err := ctx.runTurn(w, progress)
	if err != nil {
		if behavior := ctx.behaviorOf(w); behavior != nil {
			wrapped := fmt.Errorf("%s: %w", behavior.Describe(), err)
			if recordsFailure(behavior, err) {
				ctx.endFailedPerformance(behavior, wrapped)
				return moved, nil
			}
			err = wrapped
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
