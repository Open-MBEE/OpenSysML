package runtime

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// bodyRun is the work of one token's step run on a body coroutine: a breakpoint met
// inside it, or a wait on the clock, pauses the token there until stepped again.
type bodyRun struct {
	// co is the coroutine the work runs on, which a pause keeps until the work
	// has ended with err or was stopped for good.
	co    *bodyCoroutine
	work  func() error
	err   error
	after func(tokenIdx int) error
	// yield pauses the work from inside; runDepth and actionDepth are the nesting
	// the context had when the work was last resumed, which a pause gives back to.
	yield                 func(bodyPause) bool
	runDepth, actionDepth int
	// pausedAt orders the paused runs by when they last paused; paused is why.
	pausedAt int64
	paused   bodyPause
	// traceLevels is the trace nesting the paused work holds open, set aside
	// while it is paused so what runs meanwhile records at the outer depth.
	traceLevels int
	// awaitsMessages lets the run pause for a message too, as a do behavior does
	// while its machine goes on; a token's step run waits only on the clock.
	awaitsMessages bool
}

// bodyPause is why a body run paused: at the named breakpoint, or on a wait (on
// the clock, or for a message) of a flow it runs or of the executor (held) it
// performs an action with; ended reports the work done instead.
type bodyPause struct {
	breakpoint string
	onWait     bool
	held       clockWaiter
	// waits reports whether the wait goes on, so resuming would only pause again.
	waits func() bool
	ended bool
}

// bodyCoroutine runs the work of token steps one after another, so the steps of a
// run share one coroutine and only a pause, which keeps it, has the next step make another.
type bodyCoroutine struct {
	// next resumes the work, yielding why it paused or that it ended, or false once
	// the coroutine was stopped; stop ends it, and any work paused on it, for good.
	next func() (bodyPause, bool)
	stop func()
	// run is the work the coroutine is doing or has paused, nil while idle.
	run *bodyRun
}

// runPausable runs work for the token at tokenIdx, then after with the token's index
// by then, inline when a run on the stack is pausable already. The steps of a run
// share one body coroutine, kept only by a pause, as the race detector never frees one.
func (e *ActionExecutor) runPausable(tokenIdx int, work func() error, after func(tokenIdx int) error) error {
	if e.ctx.pausable != nil {
		if err := work(); err != nil {
			return err
		}
		return after(tokenIdx)
	}
	run := &bodyRun{co: e.ctx.takeBodyCoroutine(), work: work, after: after}
	e.tokens[tokenIdx].body = run
	return e.resumeBody(tokenIdx)
}

// takeBodyCoroutine takes the idle body coroutine for a step's work, making one when none is.
func (ctx *Context) takeBodyCoroutine() *bodyCoroutine {
	if co := ctx.idleBody; co != nil {
		ctx.idleBody = nil
		return co
	}
	ctx.bodyCoroutinesMade++
	co := &bodyCoroutine{}
	co.next, co.stop = iter.Pull(func(yield func(bodyPause) bool) {
		for {
			co.run.perform(ctx, yield)
			if !yield(bodyPause{ended: true}) {
				return
			}
		}
	})
	return co
}

// perform does the run's work on the coroutine resuming it, pausable through yield meanwhile.
func (run *bodyRun) perform(ctx *Context, yield func(bodyPause) bool) {
	run.yield = yield
	ctx.pausable = run
	defer func() { ctx.pausable = nil }()
	run.err = run.work()
}

// keepBodyCoroutine keeps co, whose work ended, idle for the next step; a second
// idle one is ended instead.
func (ctx *Context) keepBodyCoroutine(co *bodyCoroutine) {
	if ctx.idleBody != nil {
		co.stop()
		return
	}
	ctx.idleBody = co
}

// endIdleBodyCoroutine ends the idle body coroutine, if any, as the outermost run leaves.
func (ctx *Context) endIdleBodyCoroutine() {
	if co := ctx.idleBody; co != nil {
		ctx.idleBody = nil
		co.stop()
	}
}

// Release ends the run for good: the work of every token a breakpoint left
// paused is ended, so an executor abandoned mid-run holds no suspended run, the
// clock drives it no further, and a later Step or RunToCompletion returns
// ErrExecutorReleased, completed or not. Safe to call more than once.
func (e *ActionExecutor) Release() {
	defer e.ctx.endExecutorRun(&e.driven)()
	e.released = true
	e.endPausedBodies()
	e.ctx.clock.detach(e)
}

// endPausedBodies ends the work of every token a breakpoint left paused; a run
// that fails ends it too, as no step of it goes on.
func (e *ActionExecutor) endPausedBodies() {
	for i := range e.tokens {
		if run := e.tokens[i].body; run != nil {
			run.end(e.ctx)
			e.tokens[i].body = nil
		}
	}
}

// end ends the paused work for good, unwinding it on the nesting the context has
// now; an action the work was performing is let go of, so the clock drives it no further.
func (run *bodyRun) end(ctx *Context) {
	run.runDepth, run.actionDepth = ctx.runDepth, ctx.actionDepth
	outer := ctx.trace.nesting()
	run.co.stop()
	ctx.trace.setNesting(outer)
	if held := run.paused.held; held != nil {
		held.Release()
	}
}

// resume lets the paused work go on to its next pause, which it reports as true
// with why, or to its end, leaving err set; the coroutine is kept for the next step.
func (run *bodyRun) resume(ctx *Context) (bodyPause, bool) {
	run.runDepth, run.actionDepth = ctx.runDepth, ctx.actionDepth
	co := run.co
	co.run = run
	pausable, outer := ctx.pausable, ctx.trace.nesting()
	ctx.trace.setNesting(outer + run.traceLevels)
	pause, alive := co.next()
	ctx.pausable = pausable
	if alive && !pause.ended {
		run.paused = pause
		run.traceLevels = ctx.trace.nesting() - outer
		ctx.trace.setNesting(outer)
		return pause, true
	}
	co.run = nil
	if alive {
		ctx.keepBodyCoroutine(co)
	}
	return pause, false
}

// pausedTokens returns the IDs of the tokens whose work a breakpoint paused, the
// longest paused first.
func (e *ActionExecutor) pausedTokens() []int64 {
	var paused []Token
	for _, token := range e.tokens {
		if token.body != nil {
			paused = append(paused, token)
		}
	}
	slices.SortFunc(paused, func(a, b Token) int {
		return cmp.Compare(a.body.pausedAt, b.body.pausedAt)
	})
	ids := make([]int64, len(paused))
	for i, token := range paused {
		ids[i] = token.ID
	}
	return ids
}

// tokenIndex returns the index of the token with the given ID, -1 for none.
func (e *ActionExecutor) tokenIndex(id int64) int {
	for i, token := range e.tokens {
		if token.ID == id {
			return i
		}
	}
	return -1
}

// resumeBody lets the paused work of the token at tokenIdx go on to its next pause
// (a breakpoint suspends the executor, the clock does not) or to its end.
func (e *ActionExecutor) resumeBody(tokenIdx int) error {
	run := e.tokens[tokenIdx].body
	if pause, paused := run.resume(e.ctx); paused {
		e.pauses++
		run.pausedAt = e.pauses
		if !pause.onWait {
			e.pausedAt = pause.breakpoint
			e.state = StateSuspended
		}
		return nil
	}
	e.tokens[tokenIdx].body = nil
	if run.err != nil {
		return run.err
	}
	return run.after(tokenIdx)
}

// pauseAt pauses the run before a node a breakpoint is set on performs, as the run
// pauses before a token steps such a node; a run not made pausable goes on.
func (e *ActionExecutor) pauseAt(node ast.Node) error {
	if name := e.breakpointNameOf(node); name != "" {
		return e.ctx.pauseRun(bodyPause{breakpoint: name})
	}
	return nil
}

// pauseRun pauses the pausable run on the stack until it is resumed, giving back
// the run nesting its frames hold meanwhile; a run with none pausable goes on.
func (ctx *Context) pauseRun(pause bodyPause) error {
	run := ctx.pausable
	if run == nil {
		return nil
	}
	heldRuns, heldActions := ctx.runDepth-run.runDepth, ctx.actionDepth-run.actionDepth
	ctx.runDepth, ctx.actionDepth = run.runDepth, run.actionDepth
	ctx.pausable = nil
	resumed := run.yield(pause)
	ctx.pausable = run
	ctx.runDepth, ctx.actionDepth = run.runDepth+heldRuns, run.actionDepth+heldActions
	if !resumed {
		where := "on a wait"
		if !pause.onWait {
			where = fmt.Sprintf("at breakpoint %q", pause.breakpoint)
		}
		return fmt.Errorf("%w: the run paused %s was abandoned", ErrActionDeadlock, where)
	}
	return nil
}

// pauseForClock pauses the pausable run on the stack while held (nil for a flow of
// the run's own executor) waits on the clock, as long as waits reports; false when
// none is pausable.
func (ctx *Context) pauseForClock(held clockWaiter, waits func() bool) (bool, error) {
	if ctx.pausable == nil {
		return false, nil
	}
	return true, ctx.pauseRun(bodyPause{onWait: true, held: held, waits: waits})
}

// pauseForMessage pauses the pausable run on the stack while held (nil for a flow
// of the run's own executor) waits for a message, as long as waits reports; false
// when none is pausable or the run cannot wait for one.
func (ctx *Context) pauseForMessage(held clockWaiter, waits func() bool) (bool, error) {
	if ctx.pausable == nil || !ctx.pausable.awaitsMessages {
		return false, nil
	}
	return true, ctx.pauseRun(bodyPause{onWait: true, held: held, waits: waits})
}

// holdClock keeps the clock where it is while the behavior named runs: a wait on the
// clock under it, with no run to pause, is an error rather than an advance.
func (ctx *Context) holdClock(behavior string) func() {
	outer := ctx.clockHeldBy
	ctx.clockHeldBy = behavior
	return func() { ctx.clockHeldBy = outer }
}

// driveClock reports the error where a wait on the clock, described by waits, cannot
// advance it because a behavior on the stack holds it; nil where the clock is free.
func (ctx *Context) driveClock(waits string) error {
	if ctx.clockHeldBy == "" {
		return nil
	}
	return fmt.Errorf("%w: %s waits for the clock (%s), which only a do behavior may",
		ErrStateBehaviorWaits, ctx.clockHeldBy, waits)
}

// pausedOnClock reports a token whose work waits on the clock through a flow it
// runs or an action it performs; the tokens parked there hold the wait, not this one.
func (t Token) pausedOnClock() bool {
	return t.body != nil && t.body.paused.onWait
}

// resumable reports a token whose paused work would go on if resumed now: paused
// at a breakpoint, or on the clock for a wait that has ended.
func (t Token) resumable() bool {
	return t.body != nil && (!t.body.paused.onWait || !t.body.paused.waits())
}

// heldWaiter returns the executor performing an action for the token's paused
// work, whose wait on the clock the work waits for; nil for none.
func (t Token) heldWaiter() clockWaiter {
	if t.body == nil {
		return nil
	}
	return t.body.paused.held
}

// drivenByBody reports whether the token runs in a flow a body statement runs
// (runSubflow), whose steps that statement takes rather than Step.
func (t Token) drivenByBody() bool {
	for f := t.frame; f != nil; f = f.parent {
		if f.inBody {
			return true
		}
	}
	return false
}
