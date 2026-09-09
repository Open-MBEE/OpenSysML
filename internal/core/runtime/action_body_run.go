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
}

// bodyPause is why a body run paused: at the named breakpoint, or on the clock
// for a wait of a flow it runs or of the executor (held) it performs an action with;
// ended reports the work done instead.
type bodyPause struct {
	breakpoint string
	onClock    bool
	held       clockWaiter
	ended      bool
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

// end ends the paused work for good, unwinding it on the nesting the context has now.
func (run *bodyRun) end(ctx *Context) {
	run.runDepth, run.actionDepth = ctx.runDepth, ctx.actionDepth
	run.co.stop()
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
	run.runDepth, run.actionDepth = e.ctx.runDepth, e.ctx.actionDepth
	co := run.co
	co.run = run
	pause, alive := co.next()
	if alive && !pause.ended {
		e.pauses++
		run.pausedAt = e.pauses
		run.paused = pause
		if !pause.onClock {
			e.pausedAt = pause.breakpoint
			e.state = StateSuspended
		}
		return nil
	}
	e.tokens[tokenIdx].body = nil
	co.run = nil
	if alive {
		e.ctx.keepBodyCoroutine(co)
	}
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
		where := "on the clock"
		if !pause.onClock {
			where = fmt.Sprintf("at breakpoint %q", pause.breakpoint)
		}
		return fmt.Errorf("%w: the run paused %s was abandoned", ErrActionDeadlock, where)
	}
	return nil
}

// pauseForClock pauses the pausable run on the stack while held (nil for a flow of
// the run's own executor) waits on the clock; false when none is pausable.
func (ctx *Context) pauseForClock(held clockWaiter) (bool, error) {
	if ctx.pausable == nil {
		return false, nil
	}
	return true, ctx.pauseRun(bodyPause{onClock: true, held: held})
}

// pausedOnClock reports a token whose work waits on the clock through a flow it
// runs or an action it performs; the tokens parked there hold the wait, not this one.
func (t Token) pausedOnClock() bool {
	return t.body != nil && t.body.paused.onClock
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
