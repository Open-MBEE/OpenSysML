package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const (
	occurrencePredecessorsFQN          = occurrenceFQN + "::predecessors"
	occurrenceSuccessorsFQN            = occurrenceFQN + "::successors"
	occurrenceImmediatePredecessorsFQN = occurrenceFQN + "::immediatePredecessors"
	occurrenceImmediateSuccessorsFQN   = occurrenceFQN + "::immediateSuccessors"
)

// behaviorOccurrence is the performance occurrence of a started behavior; nil
// while it has not started.
func behaviorOccurrence(behavior *ObjectBehavior) *Instance {
	switch {
	case behavior == nil:
		return nil
	case behavior.Action != nil:
		return behavior.Action.occurrence
	case behavior.State != nil:
		return behavior.State.occurrence
	}
	return nil
}

// recordFiredSuccessions records the performances whose end let an admitted
// behavior start: its immediate predecessors (Occurrences.kerml), one per fired order.
func (ctx *Context) recordFiredSuccessions(behavior *ObjectBehavior) {
	successor := behaviorOccurrence(behavior)
	if successor == nil || behavior.member == nil {
		return
	}
	for _, order := range ctx.behaviorOrdersFor(behavior.member, behavior.Object, true) {
		if order.Refusal != nil {
			continue
		}
		for _, featuring := range ctx.behaviorOrderMatches(behavior.Object, behavior.member, order, order.Later) {
			if !ctx.laterEndIsUnique(order, featuring, nil, nil) {
				continue
			}
			predecessors := ctx.behaviorOrderEndPerformances(order, order.Earlier, featuring)
			if len(predecessors) != 1 || !ctx.behaviorPerformanceEnded(predecessors[0]) {
				continue
			}
			predecessor := behaviorOccurrence(predecessors[0])
			if predecessor == nil || slices.Contains(ctx.fired[successor.ID], predecessor.ID) {
				continue
			}
			prior := ctx.fired[successor.ID]
			ctx.fired[successor.ID] = append(slices.Clip(prior), predecessor.ID)
			ctx.noteProbeUndo(func() { ctx.fired[successor.ID] = prior })
			ctx.livesChanged()
		}
	}
}

// successionValues answers inst's immediate predecessors or successors from the
// successions that fired, and all of them from the time order; false for any other.
func (ctx *Context) successionValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	ctx.readsLives()
	var ids []int64
	var err error
	switch {
	case ctx.isLibraryMember(sym, inst.Type, occurrenceImmediatePredecessorsFQN):
		ids = ctx.fired[inst.ID]
	case ctx.isLibraryMember(sym, inst.Type, occurrencePredecessorsFQN):
		ids, err = ctx.timeOrdered(inst, true)
	case ctx.isLibraryMember(sym, inst.Type, occurrenceImmediateSuccessorsFQN):
		ids = ctx.firedSuccessors(inst.ID)
	case ctx.isLibraryMember(sym, inst.Type, occurrenceSuccessorsFQN):
		ids, err = ctx.timeOrdered(inst, false)
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	values := make([]Value, 0, len(ids))
	for _, id := range ids {
		val, err := ctx.objectValue(ctx.instances[id])
		if err != nil {
			return nil, true, err
		}
		values = append(values, val)
	}
	return values, true, nil
}

// timeOrderOpen is the undetermined read of inst's predecessors before it began
// or its successors before it ended, with the reason; false for any other member.
func (ctx *Context) timeOrderOpen(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, bool) {
	l := ctx.lives[inst.ID]
	switch {
	case ctx.isLibraryMember(sym, inst.Type, occurrencePredecessorsFQN) && l.began == 0:
		return undeterminedFeatureValue(fmt.Sprintf("%s has not started: its startShot has not happened", SymbolText(inst.Type)), feat.Multiplicity, sym), true
	case ctx.isLibraryMember(sym, inst.Type, occurrenceSuccessorsFQN) && l.ended == 0:
		return undeterminedFeatureValue(fmt.Sprintf("%s has not ended: its endShot has not happened", SymbolText(inst.Type)), feat.Multiplicity, sym), true
	}
	return Value{}, false
}

// timeOrdered lists the occurrences that ended before inst began (before) or began
// after it ended (Occurrences.kerml `predecessors`, `successors`), shots included, by id.
func (ctx *Context) timeOrdered(inst *Instance, before bool) ([]int64, error) {
	l := ctx.lives[inst.ID]
	ordered := func(began, ended int64) bool {
		if before {
			return ended != 0 && ended < l.began
		}
		return began != 0 && began > l.ended
	}
	// Every shot in the order is materialized first, so the answer does not
	// depend on which shots earlier reads happened to make.
	ids := make([]int64, 0, len(ctx.instances))
	for id := range ctx.instances {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		o := ctx.lives[id]
		if _, isShot := ctx.shotOccurrence(id); isShot || id == inst.ID {
			continue
		}
		for _, start := range [...]bool{true, false} {
			at := o.ended
			if start {
				at = o.began
			}
			if !ordered(at, at) {
				continue
			}
			other := ctx.instances[id]
			fqn := occurrenceEndShotFQN
			if start {
				fqn = occurrenceStartShotFQN
			}
			member := ctx.shotMemberOf(other.Type, ctx.librarySymbol(fqn))
			if member == nil {
				continue
			}
			feat := ctx.effectiveFeature(member.Name, member, other.Type)
			if _, err := ctx.shotOf(other, member, &feat, start); err != nil {
				return nil, err
			}
		}
	}
	out := make([]int64, 0, len(ids))
	for id := range ctx.instances {
		if o := ctx.lives[id]; id != inst.ID && ordered(o.began, o.ended) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out, nil
}

// isLibraryMember reports whether sym, a member of typ, is the library member
// fqn names or redefines it through any chain of redefinitions.
func (ctx *Context) isLibraryMember(sym, typ *symbols.Symbol, fqn string) bool {
	lib := ctx.librarySymbol(fqn)
	if lib == nil {
		return false
	}
	seen := map[*symbols.Symbol]bool{}
	var redefines func(sym *symbols.Symbol) bool
	redefines = func(sym *symbols.Symbol) bool {
		if sym == lib {
			return true
		}
		if sym == nil || seen[sym] {
			return false
		}
		seen[sym] = true
		for _, redefined := range ctx.redefinedFeatures(sym, typ) {
			if redefines(redefined) {
				return true
			}
		}
		return false
	}
	return redefines(sym)
}

// firedSuccessors are the performances the successions from id fired for, in
// the order they were recorded.
func (ctx *Context) firedSuccessors(id int64) []int64 {
	var out []int64
	for successor, predecessors := range ctx.fired {
		if slices.Contains(predecessors, id) {
			out = append(out, successor)
		}
	}
	slices.Sort(out)
	return out
}
