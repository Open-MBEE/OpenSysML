package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	if feat == nil || ctx.model.semantics == nil || !mult.Upper.Infinite && mult.Upper.Value <= 1 {
		return false
	}
	return ctx.model.semantics.HoldsSet(feat, typeSym)
}

// specializes reports whether sym is general, or has it among its supertypes.
func (ctx *Context) specializes(sym, general *symbols.Symbol) bool {
	if sym == nil || general == nil {
		return false
	}
	if sym == general {
		return true
	}
	for _, sup := range ctx.model.semantics.AllSupertypes(sym) {
		if sup == general {
			return true
		}
	}
	return false
}

// collectionOf is the collection a multi-valued feature holds the elements as.
func (ctx *Context) collectionOf(feat *EffectiveFeature, elements []Value) Value {
	if feat.HoldsSet {
		return ctx.setOf(elements)
	}
	return sequenceOf(elements)
}

// declaredCollection is a value read through a declaration, in the shape that declaration
// holds it (heldShape): one value held by an at-most-one multiplicity is held bare, one held
// by a multi-valued feature is a one-element collection, a set flowing into a sequence is its
// canonical sequence, a sequence flowing into a set its distinct elements.
func (ctx *Context) declaredCollection(sym *symbols.Symbol, val Value) (Value, error) {
	mult := ctx.featureMultiplicity(sym, nil)
	feat := &EffectiveFeature{Symbol: sym, Multiplicity: mult, HoldsSet: ctx.holdsSet(sym, ctx.findOwnerType(sym), mult)}
	return ctx.heldShape(feat, val)
}
