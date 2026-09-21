package runtime

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrOccurrenceDestroyed is returned when a destroyed occurrence is read or
	// written: its features hold nothing after its end, and it cannot end twice.
	ErrOccurrenceDestroyed = errors.New("occurrence was destroyed")

	// ErrNotAnOccurrence is returned when a value that denotes no object of the
	// model is passed where OccurrenceFunctions takes an Occurrence.
	ErrNotAnOccurrence = errors.New("value is not an occurrence")

	// ErrOccurrenceLifetime is returned when an occurrence's lifetime admits no
	// such change: `create` of one begun before the call, `destroy` of one ended.
	ErrOccurrenceLifetime = errors.New("occurrence lifetime does not admit the change")
)

// life is the lifetime of one occurrence in the runtime's execution order, kept
// beside the instances so the Kernel Semantic Library's frame stays off them.
type life struct {
	reached   int64 // the activation the runtime materialized the object at
	began     int64 // the activation the occurrence began at
	ended     int64 // the activation the occurrence ended at, 0 while it lives
	destroyed bool  // ended by `destroy`, after which its features are not read
}

// alive reports whether the occurrence has begun and not ended.
func (l life) alive() bool { return l.began > 0 && l.ended == 0 }

// readsLives lists the `=` value being derived, if any, as reading the lives: which
// objects there are, and when each began and ended.
func (ctx *Context) readsLives() { ctx.noteRead(&ctx.lifetimes) }

// livesChanged unmaterializes what derived from the lives, save what is deriving now:
// a change a `=` value makes while deriving is its own, and what it derives reflects it.
func (ctx *Context) livesChanged() {
	src := &ctx.lifetimes
	var deriving, settled []*FeatureValue
	for _, dep := range src.dependents {
		if ctx.isDeriving(dep) {
			deriving = append(deriving, dep)
		} else {
			settled = append(settled, dep)
		}
	}
	if len(settled) == 0 {
		return
	}
	ctx.noteProbeWrite(src)
	src.dependents = listDependents(deriving, ctx.invalidate(settled))
}

// isDeriving reports whether fv is being derived right now.
func (ctx *Context) isDeriving(fv *FeatureValue) bool {
	for i := range ctx.deriving {
		if ctx.deriving[i].fv == fv {
			return true
		}
	}
	return false
}

// forgetDerivedFrom unmaterializes what derived from a feature of the ended objects:
// read again, it finds them destroyed, as reading the feature itself does.
func (ctx *Context) forgetDerivedFrom(ended map[int64]bool) {
	for id := range ended {
		inst, ok := ctx.instances[id]
		if !ok {
			continue
		}
		for _, fv := range inst.FeatureValues {
			ctx.invalidateDependents(fv)
		}
	}
}

// lifeOf answers the lifetime of inst for function op; an object the context
// holds without one is a fault of the context, reported rather than guessed.
func (ctx *Context) lifeOf(op string, inst *Instance) (life, error) {
	ctx.readsLives()
	l, ok := ctx.lives[inst.ID]
	if !ok {
		return life{}, fmt.Errorf("%w: function %s: object #%d (%s) has no lifetime here",
			ErrOccurrenceLifetime, writtenName(op), inst.ID, symbolText(inst.Type))
	}
	return l, nil
}

// OccurrenceLife is the lifetime of an occurrence as the describe API reports
// it: where in the execution order it began and ended, 0 for "not yet".
type OccurrenceLife struct {
	Began int64
	Ended int64
	// Destroyed marks an end by `destroy`, after which the features hold nothing;
	// a performance that completed keeps its final values.
	Destroyed bool
}

// Alive reports whether the occurrence has begun and not ended.
func (l OccurrenceLife) Alive() bool { return l.Began > 0 && l.Ended == 0 }

// String renders the lifetime for a debugging surface.
func (l OccurrenceLife) String() string {
	switch {
	case l.Ended == 0:
		return fmt.Sprintf("alive since %d", l.Began)
	case l.Destroyed:
		return fmt.Sprintf("destroyed at %d, alive since %d", l.Ended, l.Began)
	default:
		return fmt.Sprintf("ended at %d, alive since %d", l.Ended, l.Began)
	}
}

// OccurrenceLife answers the lifetime of the occurrence an instance identity
// denotes, and false for an identity the context never registered.
func (ctx *Context) OccurrenceLife(id int64) (OccurrenceLife, bool) {
	l, ok := ctx.lives[id]
	if !ok {
		return OccurrenceLife{}, false
	}
	return OccurrenceLife{Began: l.began, Ended: l.ended, Destroyed: l.destroyed}, true
}

// beginLife records inst materialized now. A part of an object exists as long as
// its whole does: it began when its owner did and, if the owner has ended, ended
// with it, however late it is first read; anything else begins now.
func (ctx *Context) beginLife(inst *Instance) {
	now := ctx.newActivation()
	l := life{reached: now, began: now}
	if inst.owner != nil {
		if owner, ok := ctx.lives[inst.owner.ID]; ok && owner.began != 0 {
			l.began, l.ended, l.destroyed = owner.began, owner.ended, owner.destroyed
		}
	}
	ctx.lives[inst.ID] = l
	ctx.livesChanged()
}

// createDuring starts inst during the call entered at mark: only an object the
// call itself first reached can start there; one reached before it began already.
// It starts where the call reached it, ahead of the portions and performances
// reached with it, which a part that had begun with the whole holding it joins.
func (ctx *Context) createDuring(op string, inst *Instance, mark int64) error {
	prior, err := ctx.lifeOf(op, inst)
	if err != nil {
		return err
	}
	switch {
	case prior.destroyed:
		return fmt.Errorf("function %s: %w: object #%d (%s) was destroyed at %d",
			writtenName(op), ErrOccurrenceDestroyed, inst.ID, symbolText(inst.Type), prior.ended)
	case prior.reached <= mark:
		return fmt.Errorf("%w: function %s: object #%d (%s) began at %d, before the call",
			ErrOccurrenceLifetime, writtenName(op), inst.ID, symbolText(inst.Type), prior.began)
	}
	ctx.lives[inst.ID] = life{reached: prior.reached, began: prior.reached}
	ctx.noteProbeUndo(func() { ctx.lives[inst.ID] = prior })
	ctx.livesChanged()
	for _, portion := range ctx.portionsOf(inst)[1:] {
		if held := ctx.lives[portion.ID]; held.began < prior.reached {
			ctx.lives[portion.ID] = life{reached: held.reached, began: prior.reached, ended: held.ended, destroyed: held.destroyed}
			ctx.noteProbeUndo(func() { ctx.lives[portion.ID] = held })
		}
	}
	if ctx.trace != nil {
		ctx.trace.RecordOccurrenceCreated(symbolText(inst.Type), inst.ID)
	}
	return nil
}

// destroy ends inst and every object it holds as a portion of itself, none of which
// outlives its whole or ends twice, and ends the behaviors they perform; a behavior
// under way ends where its call catches the unwinding, which destroy returns.
func (ctx *Context) destroy(inst *Instance) error {
	if err := ctx.checkLiving(inst); err != nil {
		return err
	}
	// One boundary ends the whole and its portions; a portion that ended before its whole stays ended where it did.
	ended := map[int64]bool{}
	at := ctx.newActivation()
	for _, portion := range ctx.portionsOf(inst) {
		prior := ctx.lives[portion.ID]
		if prior.ended != 0 {
			continue
		}
		ended[portion.ID] = true
		ctx.lives[portion.ID] = life{reached: prior.reached, began: prior.began, ended: at, destroyed: true}
		ctx.noteProbeUndo(func() { ctx.lives[portion.ID] = prior })
		if ctx.trace != nil {
			ctx.trace.RecordOccurrenceDestroyed(symbolText(portion.Type), portion.ID)
		}
	}
	ctx.livesChanged()
	ctx.forgetDerivedFrom(ended)
	ctx.endBehaviorsWith(ended)
	ctx.forgetMessagesTo(ended)
	if ctx.innermostRun().endsWithin(ended) {
		return &terminated{object: inst, ended: ended}
	}
	return nil
}

// portionsOf lists inst and, in identity order, the objects it holds as portions of itself:
// those its composite features hold (wherever their home is) and those it is home to,
// transitively, each once however many names of a redefined feature hold it.
func (ctx *Context) portionsOf(inst *Instance) []*Instance {
	portions := []*Instance{inst}
	listed := map[int64]bool{inst.ID: true}
	for i := 0; i < len(portions); i++ {
		var owned []*Instance
		for _, fv := range portions[i].FeatureValues {
			composite := ctx.ownsHeld(fv.Feature)
			for _, element := range elementsOf(fv.HeldValue()) {
				id, ok := element.Object()
				if !ok || listed[id] {
					continue
				}
				if held, found := ctx.instances[id]; found && (composite || held.owner == portions[i]) {
					listed[id] = true
					owned = append(owned, held)
				}
			}
		}
		sort.Slice(owned, func(a, b int) bool { return owned[a].ID < owned[b].ID })
		portions = append(portions, owned...)
	}
	return portions
}

// checkPerformer refuses a destroyed or ended object as the performer of a behavior:
// an occurrence performs nothing after its end. A nil self performs outside any object.
func (ctx *Context) checkPerformer(self *Instance) error {
	if self == nil {
		return nil
	}
	if err := ctx.checkNotDestroyed(self); err != nil {
		return fmt.Errorf("performer of the behavior: %w", err)
	}
	if l, ok := ctx.lives[self.ID]; ok && l.ended != 0 {
		return fmt.Errorf("performer of the behavior: %w: object #%d (%s) ended at %d already",
			ErrOccurrenceLifetime, self.ID, symbolText(self.Type), l.ended)
	}
	return nil
}

// lifeEnded reports whether inst's lifetime here has ended; nil and an object with
// no lifetime recorded have not.
func (ctx *Context) lifeEnded(inst *Instance) bool {
	if inst == nil {
		return false
	}
	l, ok := ctx.lives[inst.ID]
	return ok && l.ended != 0
}

// checkLiving refuses an occurrence that has no lifetime here, was destroyed, or ended already.
func (ctx *Context) checkLiving(inst *Instance) error {
	prior, ok := ctx.lives[inst.ID]
	switch {
	case !ok:
		return fmt.Errorf("%w: object #%d (%s) has no lifetime here",
			ErrOccurrenceLifetime, inst.ID, symbolText(inst.Type))
	case prior.destroyed:
		return fmt.Errorf("%w: object #%d (%s) was destroyed at %d already",
			ErrOccurrenceDestroyed, inst.ID, symbolText(inst.Type), prior.ended)
	case prior.ended != 0:
		return fmt.Errorf("%w: object #%d (%s) ended at %d already",
			ErrOccurrenceLifetime, inst.ID, symbolText(inst.Type), prior.ended)
	}
	return nil
}

// completed reports whether the behavior's executor has reached its end.
func (b *ObjectBehavior) completed() bool {
	switch {
	case b.Action != nil:
		return b.Action.State().Ended()
	case b.State != nil:
		return b.State.State().Ended()
	}
	return true
}

// beginPerformanceLife records the performance occurrence inst stands for
// starting at activation, where its execution begins; nil stands for none.
func (ctx *Context) beginPerformanceLife(inst *Instance, activation int64) {
	if inst == nil {
		return
	}
	prior, ok := ctx.lives[inst.ID]
	if !ok || prior.ended != 0 {
		return
	}
	ctx.lives[inst.ID] = life{reached: prior.reached, began: activation}
	ctx.noteProbeUndo(func() { ctx.lives[inst.ID] = prior })
	ctx.livesChanged()
}

// endPerformanceLife records a performance occurrence completing, where one
// stands for the execution; one ended already (by `destroy`) stays as it is.
func (ctx *Context) endPerformanceLife(inst *Instance) {
	if inst == nil {
		return
	}
	prior, ok := ctx.lives[inst.ID]
	if !ok || prior.ended != 0 {
		return
	}
	ctx.lives[inst.ID] = life{reached: prior.reached, began: prior.began, ended: ctx.newActivation()}
	ctx.noteProbeUndo(func() { ctx.lives[inst.ID] = prior })
	ctx.livesChanged()
}

// carryLife keeps a carried-over object destroyed when it was destroyed in the
// context it came from; a living one begins anew here, as its behaviors do.
func (ctx *Context) carryLife(prev *Context, inst *Instance) {
	if prev == nil || prev == ctx {
		return
	}
	if l, ok := prev.lives[inst.ID]; ok && l.destroyed {
		here := ctx.lives[inst.ID]
		ctx.lives[inst.ID] = life{reached: here.reached, began: here.began, ended: ctx.newActivation(), destroyed: true}
		ctx.livesChanged()
	}
}

// forgetLives drops the lives of abandoned objects with the objects.
func (ctx *Context) forgetLives(abandoned map[int64]bool) {
	for id := range abandoned {
		delete(ctx.lives, id)
	}
}

// checkNotDestroyed reports a destroyed object as such, so nothing reads it as
// one still holding values.
func (ctx *Context) checkNotDestroyed(inst *Instance) error {
	if l, ok := ctx.lives[inst.ID]; ok && l.destroyed {
		return fmt.Errorf("%w: object #%d (%s) was destroyed at %d",
			ErrOccurrenceDestroyed, inst.ID, symbolText(inst.Type), l.ended)
	}
	return nil
}
