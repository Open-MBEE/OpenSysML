package runtime

import (
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

// recordFiredSuccessions records, for a behavior a succession order admitted,
// the performances whose end let it start: each is an immediate predecessor of
// the performance (Occurrences.kerml `predecessors`, `HappensBefore`), which the
// succession fired for. An order that admitted it without a completed earlier
// performance records none.
func (ctx *Context) recordFiredSuccessions(behavior *ObjectBehavior) {
	successor := behaviorOccurrence(behavior)
	if successor == nil || behavior.member == nil {
		return
	}
	seen := make(map[*Instance]bool)
	for _, order := range ctx.behaviorOrdersFor(behavior.member, behavior.Object, true) {
		if order.Refusal != nil {
			continue
		}
		for _, featuring := range ctx.behaviorOrderMatches(behavior.Object, behavior.member, order, order.Later) {
			if seen[featuring] || !ctx.laterEndIsUnique(order, featuring, nil, nil) {
				continue
			}
			seen[featuring] = true
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

// successionValues answers a collection member of inst over the successions that
// fired: its immediate predecessors or successors, or all of them through the
// chain; false for a member that is none of these.
func (ctx *Context) successionValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	ctx.readsLives()
	var ids []int64
	switch {
	case ctx.isLibraryMember(sym, inst.Type, occurrenceImmediatePredecessorsFQN):
		ids = ctx.fired[inst.ID]
	case ctx.isLibraryMember(sym, inst.Type, occurrencePredecessorsFQN):
		ids = ctx.firedClosure(inst.ID, func(id int64) []int64 { return ctx.fired[id] })
	case ctx.isLibraryMember(sym, inst.Type, occurrenceImmediateSuccessorsFQN):
		ids = ctx.firedSuccessors(inst.ID)
	case ctx.isLibraryMember(sym, inst.Type, occurrenceSuccessorsFQN):
		ids = ctx.firedClosure(inst.ID, ctx.firedSuccessors)
	default:
		return nil, false, nil
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

// firedClosure walks next from id over the fired successions, each occurrence once.
func (ctx *Context) firedClosure(id int64, next func(int64) []int64) []int64 {
	var out []int64
	seen := map[int64]bool{id: true}
	queue := []int64{id}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, n := range next(at) {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
				queue = append(queue, n)
			}
		}
	}
	return out
}
