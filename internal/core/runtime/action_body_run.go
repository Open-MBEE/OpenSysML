package runtime

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// bodyRun is the work of one token's step run as a coroutine: a breakpoint met
// inside it, or a wait on the clock, pauses the token there until stepped again.
type bodyRun struct {
	// next resumes the work, yielding why it paused next, or false once the work
	// has ended with err; stop ends paused work for good.
	next  func() (bodyPause, bool)
	stop  func()
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
// for a wait of a flow it runs or of the executor (held) it performs an action with.
type bodyPause struct {
	breakpoint string
	onClock    bool
	held       clockWaiter
	// waits reports whether the wait goes on, so resuming would only pause again.
	waits func() bool
}

// runPausable runs work for the token at tokenIdx, then after with the token's
// index by then; the work is a coroutine unless a run on the stack is pausable already.
func (e *ActionExecutor) runPausable(tokenIdx int, work func() error, after func(tokenIdx int) error) error {
	if e.ctx.pausable != nil {
		if err := work(); err != nil {
			return err
		}
		return after(tokenIdx)
	}
	run := &bodyRun{after: after}
	run.next, run.stop = iter.Pull(func(yield func(bodyPause) bool) {
		run.yield = yield
		e.ctx.pausable = run
		defer func() { e.ctx.pausable = nil }()
		run.err = work()
	})
	e.tokens[tokenIdx].body = run
	return e.resumeBody(tokenIdx)
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
	run.stop()
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
	if pause, paused := run.next(); paused {
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
// the run's own executor) waits on the clock, as long as waits reports; false when
// none is pausable.
func (ctx *Context) pauseForClock(held clockWaiter, waits func() bool) (bool, error) {
	if ctx.pausable == nil {
		return false, nil
	}
	return true, ctx.pauseRun(bodyPause{onClock: true, held: held, waits: waits})
}

// pausedOnClock reports a token whose work waits on the clock through a flow it
// runs or an action it performs; the tokens parked there hold the wait, not this one.
func (t Token) pausedOnClock() bool {
	return t.body != nil && t.body.paused.onClock
}

// resumable reports a token whose paused work would go on if resumed now: paused
// at a breakpoint, or on the clock for a wait that has ended.
func (t Token) resumable() bool {
	return t.body != nil && (!t.body.paused.onClock || !t.body.paused.waits())
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
