package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// terminateOccurrence ends the occurrence s names (SysML v2 §7.17.10): its lifetime
// and those of the objects it holds end now, with every behavior they perform. The
// body unwinds where its own executor's performance is among them.
func (e *performances) terminateOccurrence(engine *stmtEngine, s lower.Effect) error {
	text := e.ctx.bindingExprText(s.TargetExpr, s.Scope)
	target, err := e.terminateTargetOf(engine, s, text)
	if err != nil {
		return err
	}
	ended, err := e.ctx.endOccurrence(target)
	if err != nil {
		return fmt.Errorf("%w: 'terminate %s': %w", ErrTerminateOccurrence, text, err)
	}
	if e.flow.endsWith(ended) {
		return &terminated{object: target, ended: ended}
	}
	return nil
}

// terminateTargetOf evaluates the occurrence expression of s, spelled text, in the
// frames of the statement stating it; a value that is no object held here is refused.
func (e *performances) terminateTargetOf(engine *stmtEngine, s lower.Effect, text string) (*Instance, error) {
	value, err := engine.evalIn(s.Scope).Eval(s.TargetExpr)
	if err != nil {
		return nil, fmt.Errorf("%w: 'terminate %s': %w", ErrTerminateTarget, text, err)
	}
	id, ok := value.Object()
	if !ok {
		return nil, fmt.Errorf("%w: 'terminate %s' names a %s: %w",
			ErrTerminateOccurrence, text, value.Kind, ErrNotAnOccurrence)
	}
	inst, found := e.ctx.Instance(id)
	if !found {
		return nil, fmt.Errorf("%w: 'terminate %s' names object #%d, which this context does not hold: %w",
			ErrTerminateOccurrence, text, id, ErrOccurrenceLifetime)
	}
	return inst, nil
}

// endOccurrence ends inst's lifetime now, and with it the objects it holds and the
// behaviors any of them performs; it returns the identities ended. An occurrence
// that is not ongoing is refused.
func (ctx *Context) endOccurrence(inst *Instance) (map[int64]bool, error) {
	if err := ctx.checkLiving(inst); err != nil {
		return nil, err
	}
	if tr := ctx.trace; tr != nil {
		tr.RecordOccurrenceTerminated(symbolText(inst.Type), inst.ID)
	}
	ended := make(map[int64]bool)
	// One boundary ends the whole and its portions: none outlives its whole.
	at := ctx.newActivation()
	for _, portion := range ctx.portionsOf(inst) {
		if l, ok := ctx.lives[portion.ID]; ok && l.ended != 0 {
			continue
		}
		ended[portion.ID] = true
		ctx.endLifeAt(portion, at)
	}
	ctx.endBehaviorsWith(ended)
	return ended, nil
}

// endLifeAt records inst's lifetime ending at the activation at, undone with a probe.
func (ctx *Context) endLifeAt(inst *Instance, at int64) {
	prior := ctx.lives[inst.ID]
	ctx.lives[inst.ID] = life{reached: prior.reached, began: prior.began, ended: at}
	ctx.noteProbeUndo(func() { ctx.lives[inst.ID] = prior })
}

// endBehaviorsWith ends, in the order the behaviors were begun, every behavior whose
// performance ends with the objects ended and that no call under way is running;
// one under way ends where its call catches the unwinding.
func (ctx *Context) endBehaviorsWith(ended map[int64]bool) {
	for _, b := range ctx.objectBehaviors {
		if b.completed() {
			continue
		}
		switch {
		case b.Action != nil && b.Action.endsWith(ended) && !ctx.underWay(&b.Action.driven):
			b.Action.endTerminated()
		case b.State != nil && b.State.endsWith(ended) && !ctx.underWay(&b.State.driven):
			b.State.endTerminated()
		}
	}
}

// endsWith reports whether the executor's performance ends with the objects ended:
// its occurrence or performer is among them, or the performance it was begun under ends.
func (e *ActionExecutor) endsWith(ended map[int64]bool) bool {
	if e.occurrence != nil && ended[e.occurrence.ID] || e.self != nil && ended[e.self.ID] {
		return true
	}
	return e.driven.caller.endsWithin(ended)
}

// performerEnded reports whether the action's occurrence or performer ended.
func (e *ActionExecutor) performerEnded() bool {
	return e.ctx.lifeEnded(e.occurrence) || e.ctx.lifeEnded(e.self)
}

// endTerminated ends the action's performance where it is: every token is dropped
// with the bodies and performances it held, and the run is terminated.
func (e *ActionExecutor) endTerminated() {
	if e.state.Ended() {
		return
	}
	e.endPausedBodies()
	e.dropTokensIn(e.root, 0)
	endNested(e.root)
	e.root.ended, e.root.live = true, 0
	e.state = StateTerminated
	e.ctx.endPerformanceLife(e.occurrence)
}

// endedByOccurrence settles the unwinding of a terminate of an occurrence at this
// executor's step: one whose performance ends with it ends here, and the unwinding
// goes on only while the performance it was begun under ends too.
func (e *ActionExecutor) endedByOccurrence(t *terminated, err error) error {
	if !e.endsWith(t.ended) {
		return err
	}
	e.endTerminated()
	if e.driven.caller.endsWithin(t.ended) {
		return err
	}
	return nil
}

// endsWith reports whether the machine's performance ends with the objects ended:
// its occurrence or the object exhibiting it is among them, or the performance it
// was begun under ends.
func (e *StateExecutor) endsWith(ended map[int64]bool) bool {
	if e.occurrence != nil && ended[e.occurrence.ID] || e.self != nil && ended[e.self.ID] {
		return true
	}
	return e.driven.caller.endsWithin(ended)
}

// performerEnded reports whether the machine's occurrence or exhibiting object ended.
func (e *StateExecutor) performerEnded() bool {
	return e.ctx.lifeEnded(e.occurrence) || e.ctx.lifeEnded(e.self)
}

// endTerminated ends the machine's performance where it is, as a transition to a
// terminate action does: no state is exited, the do behaviors under way are abandoned.
func (e *StateExecutor) endTerminated() {
	if e.state.Ended() {
		return
	}
	abandoned := e.abandonMachine()
	if e.trace() != nil {
		e.trace().RecordStateEndedWithOccurrence(symbolText(e.stateMachine), abandoned)
	}
	e.state = StateTerminated
	e.ctx.endPerformanceLife(e.occurrence)
}

// endedByOccurrence settles the unwinding of a terminate of an occurrence at a call
// into the machine: one whose performance ends with it ends here, and the unwinding
// goes on only while the performance it was begun under ends too.
func (e *StateExecutor) endedByOccurrence(err *error) {
	t := unwound(*err)
	if t == nil || t.perf != nil || !e.endsWith(t.ended) {
		return
	}
	e.endTerminated()
	if !e.driven.caller.endsWithin(t.ended) {
		*err = nil
	}
}
