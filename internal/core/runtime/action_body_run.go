package runtime

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// bodyRun is the work of one token's step run as a coroutine, so that a breakpoint
// met inside it — at a node of a block's flow, or a step of a flow a body runs —
// pauses the token there and a later step resumes the work where it stopped.
type bodyRun struct {
	// next resumes the work, yielding the breakpoint it paused at next, or false
	// once the work has ended with err; stop ends paused work for good.
	next  func() (string, bool)
	stop  func()
	err   error
	after func(tokenIdx int) error
	// pausedAt orders the paused runs by when they last paused.
	pausedAt int64
}

// runPausable runs work for the token at tokenIdx, then after with the token's
// index by then. With a breakpoint set, work runs as a coroutine a breakpoint met
// inside pauses: the token stays where it is, its performance half done, until it
// is stepped again. Without one, or inside a run already pausable, it runs in place.
func (e *ActionExecutor) runPausable(tokenIdx int, work func() error, after func(tokenIdx int) error) error {
	if len(e.breakpoints) == 0 || e.pause != nil {
		if err := work(); err != nil {
			return err
		}
		return after(tokenIdx)
	}
	run := &bodyRun{after: after}
	run.next, run.stop = iter.Pull(func(yield func(string) bool) {
		e.pause = yield
		defer func() { e.pause = nil }()
		run.err = work()
	})
	e.tokens[tokenIdx].body = run
	return e.resumeBody(tokenIdx)
}

// Release ends the run for good: the work of every token a breakpoint left
// paused is ended, so an executor abandoned mid-run holds no suspended run, and a
// later Step or RunToCompletion returns ErrExecutorReleased, completed or not.
// Safe to call more than once.
func (e *ActionExecutor) Release() {
	defer e.ctx.beginExecutorRun(&e.driven)()
	e.released = true
	e.endPausedBodies()
}

// endPausedBodies ends the work of every token a breakpoint left paused; a run
// that fails ends it too, as no step of it goes on.
func (e *ActionExecutor) endPausedBodies() {
	for i := range e.tokens {
		if run := e.tokens[i].body; run != nil {
			run.stop()
			e.tokens[i].body = nil
		}
	}
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

// resumeBody lets the paused work of the token at tokenIdx go on to its next
// pause, or to its end, when what follows the work runs.
func (e *ActionExecutor) resumeBody(tokenIdx int) error {
	run := e.tokens[tokenIdx].body
	if name, paused := run.next(); paused {
		e.pausedAt = name
		e.state = StateSuspended
		e.pauses++
		run.pausedAt = e.pauses
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
		return e.pauseRun(name)
	}
	return nil
}

// pauseRun pauses a pausable run at the named breakpoint until it is resumed.
// While it is paused, another token's step is pausable on its own.
func (e *ActionExecutor) pauseRun(name string) error {
	pause := e.pause
	if pause == nil {
		return nil
	}
	e.pause = nil
	resumed := pause(name)
	e.pause = pause
	if !resumed {
		return fmt.Errorf("%w: the run paused at breakpoint %q was abandoned", ErrActionDeadlock, name)
	}
	return nil
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
