package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The library Collections declare `elements` once as nonunique, at the root,
// and redefine it unique (UniqueCollection, Map) or ordered (OrderedCollection).
const (
	collectionType            = "Collections::Collection"
	collectionElementsName    = "elements"
	collectionElementsFeature = collectionType + "::" + collectionElementsName
	orderedCollectionType     = "Collections::OrderedCollection"
)

// holdsSet reports whether a multi-valued feature of typeSym holds a set rather
// than a sequence. That is `elements` of a library Collection whose own
// redefinition of it is unique and not ordered — Set's and Map's, not Bag's,
// which inherits the nonunique root, and not OrderedSet's or OrderedMap's —
// unless the feature itself is declared ordered or nonunique.
func (ctx *Context) holdsSet(feat, typeSym *symbols.Symbol, mult semantics.Range) bool {
	if feat == nil || ctx.model.semantics == nil || !mult.Upper.Infinite && mult.Upper.Value <= 1 {
		return false
	}
	if declaredOrderedOrNonunique(feat) {
		return false
	}
	root := ctx.librarySymbol(collectionElementsFeature)
	if root == nil || !ctx.specializes(feat, root) {
		return false
	}
	if ctx.specializes(typeSym, ctx.librarySymbol(orderedCollectionType)) {
		return false
	}
	redefined := ctx.libraryDeclared(feat) && feat != root
	for _, sup := range ctx.model.semantics.AllSupertypes(feat) {
		if sup == root || !ctx.libraryDeclared(sup) || !ctx.specializes(sup, root) {
			continue
		}
		if declaredOrderedOrNonunique(sup) {
			return false
		}
		redefined = true
	}
	return redefined
}

// declaredOrderedOrNonunique reports whether a feature's own declaration says
// ordered or nonunique.
func declaredOrderedOrNonunique(feat *symbols.Symbol) bool {
	usage, ok := feat.Decl.(*ast.Usage)
	return ok && (usage.IsOrdered || usage.IsNonunique)
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
		return ctx.collectionOf(&EffectiveFeature{HoldsSet: holds}, elementsOf(val))
	}
	return val
}
