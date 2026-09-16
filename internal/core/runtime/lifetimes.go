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

// lifeOf answers the lifetime of inst for function op; an object the context
// holds without one is a fault of the context, reported rather than guessed.
func (ctx *Context) lifeOf(op string, inst *Instance) (life, error) {
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

// beginLife records inst materialized now. A part of an object exists as long
// as its whole does, so it began when its owner did; anything else begins now.
func (ctx *Context) beginLife(inst *Instance) {
	now := ctx.newActivation()
	l := life{reached: now, began: now}
	if inst.owner != nil {
		if owner, ok := ctx.lives[inst.owner.ID]; ok && owner.began != 0 {
			l.began = owner.began
		}
	}
	ctx.lives[inst.ID] = l
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

// destroy ends inst and every object it holds as a portion of itself, none of
// which outlives its whole, ends twice, or ends while performing a behavior.
func (ctx *Context) destroy(inst *Instance) error {
	if err := ctx.checkMayEnd(inst); err != nil {
		return err
	}
	// A portion that ended before its whole stays ended where it did.
	var portions []*Instance
	for _, portion := range ctx.portionsOf(inst) {
		if ctx.lives[portion.ID].ended != 0 {
			continue
		}
		if err := ctx.checkMayEnd(portion); err != nil {
			return err
		}
		portions = append(portions, portion)
	}
	// One boundary ends the whole and its portions: none outlives its whole.
	ended := ctx.newActivation()
	for _, portion := range portions {
		prior := ctx.lives[portion.ID]
		ctx.lives[portion.ID] = life{reached: prior.reached, began: prior.began, ended: ended, destroyed: true}
		ctx.noteProbeUndo(func() { ctx.lives[portion.ID] = prior })
		if ctx.trace != nil {
			ctx.trace.RecordOccurrenceDestroyed(symbolText(portion.Type), portion.ID)
		}
	}
	return nil
}

// portionsOf lists inst and, in identity order, the objects it holds as
// portions of itself: those its feature values own, transitively, each once
// however many names of a redefined feature hold it.
func (ctx *Context) portionsOf(inst *Instance) []*Instance {
	portions := []*Instance{inst}
	listed := map[int64]bool{inst.ID: true}
	for i := 0; i < len(portions); i++ {
		var owned []*Instance
		for _, fv := range portions[i].FeatureValues {
			for _, element := range elementsOf(fv.HeldValue()) {
				id, ok := element.Object()
				if !ok || listed[id] {
					continue
				}
				if held, found := ctx.instances[id]; found && held.owner == portions[i] {
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

// checkPerformer refuses a destroyed object as the performer of a behavior: an
// occurrence performs nothing after its end. A nil self performs outside any object.
func (ctx *Context) checkPerformer(self *Instance) error {
	if self == nil {
		return nil
	}
	if err := ctx.checkNotDestroyed(self); err != nil {
		return fmt.Errorf("performer of the behavior: %w", err)
	}
	return nil
}

// checkMayEnd reports why inst cannot end now: it ended already, or a behavior
// it performs, or one performed as it, is under way.
func (ctx *Context) checkMayEnd(inst *Instance) error {
	if err := ctx.checkLiving(inst); err != nil {
		return err
	}
	if b := ctx.performanceUnderWay(inst); b != nil {
		return fmt.Errorf("%w: object #%d (%s) cannot end while %s is under way",
			ErrOccurrenceLifetime, inst.ID, symbolText(inst.Type), b.Describe())
	}
	return nil
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

// performanceUnderWay answers a behavior inst performs, or one whose performance
// occurrence inst is, that has not completed; nil when there is none.
func (ctx *Context) performanceUnderWay(inst *Instance) *ObjectBehavior {
	performers := []*Instance{inst}
	if inst.owner != nil {
		performers = append(performers, inst.owner)
	}
	for _, performer := range performers {
		for _, b := range performer.behaviors {
			if b.performanceOf(inst) && !b.completed() {
				return b
			}
		}
	}
	return nil
}

// performanceOf reports whether the behavior is performed by inst, or is the
// performance inst stands for.
func (b *ObjectBehavior) performanceOf(inst *Instance) bool {
	if b.Object == inst {
		return true
	}
	switch {
	case b.Action != nil:
		return b.Action.occurrence == inst
	case b.State != nil:
		return b.State.occurrence == inst
	}
	return false
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
