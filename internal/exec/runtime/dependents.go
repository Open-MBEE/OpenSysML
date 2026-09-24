package runtime

// A `=` value lists itself as a dependent of each feature value it reads, and a
// change to one unmaterializes its dependents, transitively, to derive again when read.

// derivable reports whether fv's value is derived from what it reads: bound by `=`,
// neither a fallback default nor assigned by a run nor propagated by a binding.
func (ctx *Context) derivable(fv *FeatureValue) bool {
	return !fv.Written && !fv.BindingDerived && !fv.Feature.DefaultIsFallback()
}

// derivation is a `=` value being derived; stale once a value it read was written
// under it (by a behavior a read started), so its result is derived again.
type derivation struct {
	fv    *FeatureValue
	stale bool
}

// deriveFeatureValue evaluates fv's `=` expression, recording what it reads, over
// again while a read wrote under it; the step limit bounds a run that never settles.
func (ctx *Context) deriveFeatureValue(inst *Instance, fv *FeatureValue, name string) (Value, error) {
	if !ctx.derivable(fv) {
		return ctx.evalFeatureValueDefault(inst, fv, name)
	}
	for {
		ctx.forgetReads(fv)
		val, stale, err := ctx.deriveOnce(inst, fv, name)
		if err != nil || !stale {
			return val, err
		}
	}
}

// forgetReads delists fv from what its last derivation read: this one reads afresh,
// and a branch it no longer takes must not unmaterialize it.
func (ctx *Context) forgetReads(fv *FeatureValue) {
	if len(fv.reads) == 0 {
		return
	}
	ctx.noteProbeWrite(fv)
	reads := fv.reads
	fv.reads = nil
	for _, src := range reads {
		ctx.delist(src, fv)
	}
}

// delist drops dep from src's dependents.
func (ctx *Context) delist(src, dep *FeatureValue) {
	if kept := without(src.dependents, dep); len(kept) != len(src.dependents) {
		ctx.noteProbeWrite(src)
		src.dependents = kept
	}
}

// delistRead drops src from what dep reads.
func (ctx *Context) delistRead(dep, src *FeatureValue) {
	if kept := without(dep.reads, src); len(kept) != len(dep.reads) {
		ctx.noteProbeWrite(dep)
		dep.reads = kept
	}
}

// without is list less fv, copied rather than written in place, since a journal
// snapshot shares the array; list itself when fv is not in it.
func without(list []*FeatureValue, fv *FeatureValue) []*FeatureValue {
	for i, listed := range list {
		if listed != fv {
			continue
		}
		if len(list) == 1 {
			return nil
		}
		kept := make([]*FeatureValue, 0, len(list)-1)
		return append(append(kept, list[:i]...), list[i+1:]...)
	}
	return list
}

// forgetEdgesOf drops every edge into or out of the feature values of objects a
// failed creation abandons; what read one derives again from the object built instead.
func (ctx *Context) forgetEdgesOf(objects []*Instance) {
	for _, inst := range objects {
		for _, fv := range inst.FeatureValues {
			ctx.forgetReads(fv)
			for _, dep := range fv.dependents {
				ctx.delistRead(dep, fv)
			}
			ctx.invalidate(fv.dependents)
			fv.dependents = nil
		}
	}
}

// deriveOnce derives fv under a frame of its own, reporting whether it went stale.
func (ctx *Context) deriveOnce(inst *Instance, fv *FeatureValue, name string) (val Value, stale bool, err error) {
	top := len(ctx.deriving)
	ctx.deriving = append(ctx.deriving, derivation{fv: fv})
	defer func() {
		stale = ctx.deriving[top].stale
		ctx.deriving = ctx.deriving[:top]
	}()
	val, err = ctx.evalFeatureValueDefault(inst, fv, name)
	return val, false, err
}

// noteRead lists the value being derived, if any, as a dependent of the fv just read,
// and fv among what it reads.
func (ctx *Context) noteRead(fv *FeatureValue) {
	if len(ctx.deriving) == 0 {
		return
	}
	dep := ctx.deriving[len(ctx.deriving)-1].fv
	if dep == fv {
		return
	}
	for _, listed := range fv.dependents {
		if listed == dep {
			return
		}
	}
	ctx.noteProbeWrite(fv)
	ctx.noteProbeWrite(dep)
	fv.dependents = append(fv.dependents, dep)
	dep.reads = append(dep.reads, fv)
}

// held is what a feature value held before a write, with what depended on it then.
type held struct {
	val          Value
	materialized bool
	taken        bool
	dependents   []*FeatureValue
}

// beforeWrite takes what fv holds, setting its dependents aside until afterWrite: the
// writes on the way (a binding cleared and bound again) count as one, by the result.
func (ctx *Context) beforeWrite(fv *FeatureValue) held {
	if fv.changing || (len(fv.dependents) == 0 && len(ctx.deriving) == 0) {
		return held{}
	}
	ctx.noteProbeWrite(fv)
	p := held{val: fv.HeldValue(), materialized: fv.Materialized, taken: true, dependents: fv.dependents}
	fv.dependents, fv.changing = nil, true
	return p
}

// afterWrite unmaterializes what depended on fv before the write unless fv holds
// what it held; what read fv during the write read what it holds now and stays listed.
func (ctx *Context) afterWrite(fv *FeatureValue, p held) {
	if !p.taken {
		return
	}
	ctx.noteProbeWrite(fv)
	fv.changing = false
	if p.dependents == nil {
		return
	}
	if p.materialized != fv.Materialized || (fv.Materialized && !heldSame(p.val, fv.HeldValue())) {
		p.dependents = ctx.invalidate(p.dependents)
	}
	fv.dependents = listDependents(fv.dependents, p.dependents)
}

// listDependents lists each of more not already listed, without copying when nothing is.
func listDependents(listed, more []*FeatureValue) []*FeatureValue {
	if len(listed) == 0 {
		return more
	}
next:
	for _, dep := range more {
		for _, have := range listed {
			if have == dep {
				continue next
			}
		}
		listed = append(listed, dep)
	}
	return listed
}

// heldSame reports whether now is prior as a derivation reads it: `===`, and for a
// quantity the same unit too, which an operation over it carries into its result.
func heldSame(prior, now Value) bool {
	if isEmptyValue(prior) || isEmptyValue(now) {
		return isEmptyValue(prior) && isEmptyValue(now) && emptyUnitSame(prior, now)
	}
	if prior.Kind != now.Kind {
		return false
	}
	switch prior.Kind {
	case ValInvalid:
		// Holding nothing, as an optional part with no object does.
		return true
	case ValQuantity:
		return prior.Quantity().Unit.Product.Equal(now.Quantity().Unit.Product) && valueIdentical(prior, now)
	case ValVectorQuantity:
		return vectorQuantityHeldSame(prior.VectorQuantity(), now.VectorQuantity())
	case ValTensorQuantity:
		return tensorQuantityHeldSame(prior.TensorQuantity(), now.TensorQuantity())
	case ValSequence:
		return elementsHeldSame(prior.Sequence().Elements(), now.Sequence().Elements())
	case ValArray:
		return valueIdentical(prior, now) && elementsHeldSame(prior.Array().Elements, now.Array().Elements)
	case ValSet:
		return setHeldSame(prior.Set(), now.Set())
	}
	return valueIdentical(prior, now)
}

// emptyUnitSame reports whether two empty values type the zero a sum of them
// yields alike: both in one unit, or neither in any.
func emptyUnitSame(prior, now Value) bool {
	pu, pok := prior.Sequence().ElementUnit()
	nu, nok := now.Sequence().ElementUnit()
	if pok != nok {
		return false
	}
	return !pok || pu.Product.Equal(nu.Product)
}

func vectorQuantityHeldSame(prior, now *VectorQuantity) bool {
	if prior == nil || now == nil || prior.Dimension() != now.Dimension() {
		return prior == now
	}
	if !sameVectorFrame(prior, now) {
		return false
	}
	for i := range prior.Num {
		if !heldSame(NewQuantityValue(prior.component(i)), NewQuantityValue(now.component(i))) {
			return false
		}
	}
	return true
}

func elementsHeldSame(prior, now []Value) bool {
	if len(prior) != len(now) {
		return false
	}
	for i := range prior {
		if !heldSame(prior[i], now[i]) {
			return false
		}
	}
	return true
}

func setHeldSame(prior, now *Set) bool {
	if prior == nil || now == nil || prior.Size() != now.Size() {
		return prior == now
	}
	rights := now.Elements()
	used := make([]bool, len(rights))
	for _, left := range prior.Elements() {
		found := false
		for i, right := range rights {
			if !used[i] && heldSame(left, right) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// invalidateDependents unmaterializes what was derived from fv, transitively. The
// list is detached before it is walked, so a cycle ends the walk.
func (ctx *Context) invalidateDependents(fv *FeatureValue) {
	if len(fv.dependents) == 0 {
		return
	}
	ctx.noteProbeWrite(fv)
	dependents := fv.dependents
	fv.dependents = nil
	fv.dependents = ctx.invalidate(dependents)
}

// invalidate unmaterializes the dependents, transitively, returning those being
// derived right now: each stays listed, its derivation stale as its source changed under it.
func (ctx *Context) invalidate(dependents []*FeatureValue) (deriving []*FeatureValue) {
	for _, dep := range dependents {
		switch {
		case ctx.markStale(dep):
			deriving = append(deriving, dep)
		case dep.Written || dep.BindingDerived:
			// Assigned or bound since: its value is no longer derived from fv.
		case !dep.Materialized:
			ctx.invalidateDependents(dep)
		default:
			ctx.noteProbeWrite(dep)
			dep.Value, dep.Values, dep.Materialized = Value{}, Value{}, false
			ctx.invalidateDependents(dep)
		}
	}
	return deriving
}

// markStale reports whether fv is being derived right now, marking that derivation stale.
func (ctx *Context) markStale(fv *FeatureValue) bool {
	for i := range ctx.deriving {
		if ctx.deriving[i].fv == fv {
			ctx.deriving[i].stale = true
			return true
		}
	}
	return false
}
