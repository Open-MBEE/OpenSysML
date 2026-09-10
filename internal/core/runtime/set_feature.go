package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The library Collections declare `elements` once as nonunique, at the root,
// and redefine it unique (UniqueCollection, Map) or ordered (OrderedCollection).
const (
	collectionType         = "Collections::Collection"
	collectionElementsName = "elements"
)

// holdsSet reports whether a multi-valued feature of typeSym holds a set rather
// than a sequence (semantics.Model.HoldsSet); a single-valued feature holds neither.
func (ctx *Context) holdsSet(feat, typeSym *symbols.Symbol, mult semantics.Range) bool {
	if feat == nil || ctx.model == nil || !mult.Upper.Infinite && mult.Upper.Value <= 1 {
		return false
	}
	return ctx.model.HoldsSet(feat, typeSym)
}

// specializes reports whether sym is general, or has it among its supertypes.
func (ctx *Context) specializes(sym, general *symbols.Symbol) bool {
	if sym == nil || general == nil {
		return false
	}
	if sym == general {
		return true
	}
	for _, sup := range ctx.model.AllSupertypes(sym) {
		if sup == general {
			return true
		}
	}
	return false
}

// collectionOf is the collection a multi-valued feature holds the elements as.
func collectionOf(feat *EffectiveFeature, elements []Value) Value {
	if feat.HoldsSet {
		return setOf(elements)
	}
	return sequenceOf(elements)
}

// declaredCollection is a collection read through a multi-valued declaration, as
// the kind that declaration holds: a set flowing into a sequence is its canonical
// sequence, a sequence flowing into a set its distinct elements.
func (ctx *Context) declaredCollection(sym *symbols.Symbol, val Value) Value {
	if val.Kind != ValSet && val.Kind != ValSequence {
		return val
	}
	mult := ctx.featureMultiplicity(sym, nil)
	if !mult.Upper.Infinite && mult.Upper.Value <= 1 {
		return val
	}
	if holds := ctx.holdsSet(sym, ctx.findOwnerType(sym), mult); holds != (val.Kind == ValSet) {
		return collectionOf(&EffectiveFeature{HoldsSet: holds}, elementsOf(val))
	}
	return val
}
