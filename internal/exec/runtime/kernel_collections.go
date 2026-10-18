package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kernelCollectionRead answers a read of a Kernel Semantic Library collection the
// object's shape leaves out (`suboccurrences`, `subobjects`, `subperformances`, …):
// the values of a subsetting feature are values of the feature it subsets (KerML 1.0
// §7.3.4.4), so the collection holds the values of the shape features that subset it,
// directly or through what they redefine, in shape order. The second result reports
// whether name is such a collection; a name the runtime tracks itself or that is no
// collection is left to the caller.
func (ctx *Context) kernelCollectionRead(inst *Instance, name string, open *openPopulation) (*FeatureValue, bool, error) {
	sym, ok := ctx.kernelCollection(inst.Type, name)
	if !ok {
		return nil, false, nil
	}
	fv, err := ctx.kernelCollectionValue(inst, sym, open)
	return fv, true, err
}

// kernelCollection is the Kernel-tier collection feature of typ that name reads: a
// frame-tier usage of multiplicity `[0..*]` whose values the library states by nothing
// but subsetting. One that is `inverse of` or `unions` another, intersects one it does
// not subset, holds a
// value of its own or has a lower bound (`timeEnclosedOccurrences [1..*]`, which holds the
// occurrence itself) has values the shape cannot supply, as has a frame root the runtime
// tracks itself; those are left to the caller.
func (ctx *Context) kernelCollection(typ *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if ctx == nil || typ == nil {
		return nil, false
	}
	sym, found := ctx.model.semantics.LookupMember(typ, name)
	if !found || sym == nil || !sym.DeclaresUsage() || !ctx.libraryTier(sym).Frame() {
		return nil, false
	}
	sem := ctx.model.semantics
	if sem.FrameRoot(typ, sym) || semantics.IsParameter(sym) || sem.HeldByValue(sym) {
		return nil, false
	}
	for _, kind := range []ast.RelationshipKind{ast.RelInverseOf, ast.RelUnions} {
		if len(relationshipsOfKind(sym, kind)) > 0 {
			return nil, false
		}
	}
	if !ctx.intersectsOnlySubsetted(sym) || ctx.extractDefaultValue(sym) != nil {
		return nil, false
	}
	mult := ctx.featureMultiplicity(sym, typ)
	if !mult.Upper.Infinite && (!mult.Upper.Known || mult.Upper.Value <= 1) {
		return nil, false
	}
	if !mult.Lower.Known || mult.Lower.Infinite || mult.Lower.Value != 0 {
		return nil, false
	}
	return sym, true
}

// derivesKernelCollection reports whether a read of name on inst, absent from its
// feature values, is answered by kernelCollectionRead.
func (ctx *Context) derivesKernelCollection(inst *Instance, name string) bool {
	if inst == nil {
		return false
	}
	_, ok := ctx.kernelCollection(inst.Type, name)
	return ok
}

// kernelCollectionValue is the collection sym holds on inst: the elements of every
// shape feature subsetting it, each object once unless sym is nonunique. It is
// derived on each read and stored nowhere, so the shape stays as declared. A
// model-level read (open non-nil) that stops short of making up a subsetter's
// count leaves the collection open too, certainly holding what every subsetter
// certainly holds and no fewer than the most any one of them holds.
func (ctx *Context) kernelCollectionValue(inst *Instance, sym *symbols.Symbol, open *openPopulation) (*FeatureValue, error) {
	feat := ctx.effectiveFeature(sym.Name, sym, inst.Type)
	var values []Value
	var seen map[int64]bool
	stopped, atLeast := false, int64(0)
	add := func(elements []Value) {
		if feat.Unique {
			values, seen = appendUniqueInstances(values, elements, seen)
		} else {
			values = append(values, elements...)
		}
	}
	features := ctx.FeaturesOf(inst.Type)
	for i := range features {
		shape := &features[i]
		if shape.Symbol == nil || !ctx.subsetsTransitively(shape.Symbol, inst.Type, sym) {
			continue
		}
		var sub *openPopulation
		if open != nil {
			sub = &openPopulation{}
		}
		fv, err := inst.getFeatureValue(ctx, shape.Name, sub)
		if err != nil {
			return nil, err
		}
		if sub != nil && sub.Stopped {
			stopped = true
			atLeast = max(atLeast, int64(len(sub.Contributed)), sub.AtLeast, fewestOf(shape.Multiplicity))
			add(sub.Contributed)
			continue
		}
		held, err := ctx.HeldElements(fv.HeldValue())
		if err != nil {
			return nil, err
		}
		add(held)
	}
	if stopped {
		open.Stopped, open.Contributed, open.AtLeast = true, values, atLeast
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, err
	}
	return &FeatureValue{Feature: &feat, Values: ctx.collectionOf(&feat, values), Materialized: !stopped}, nil
}

// subsetsTransitively reports whether feature, a member of owner, subsets target
// through any number of declared `:>` edges, implicit nesting subsettings and
// redefinitions, in whatever order they alternate.
func (ctx *Context) subsetsTransitively(feature, owner, target *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{feature: true}
	for queue := []*symbols.Symbol{feature}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		next := ctx.relatedFeatures(cur, owner, ast.RelSubsets)
		next = append(next, ctx.redefinedFeatures(cur, owner)...)
		if ctx.libraryTier(cur) == symbols.TierNone {
			next = append(next, ctx.model.semantics.ImplicitSubsettings(cur)...)
		}
		for _, sub := range next {
			if sub == target {
				return true
			}
			if sub != nil && !seen[sub] {
				seen[sub] = true
				queue = append(queue, sub)
			}
		}
	}
	return false
}

// intersectsOnlySubsetted reports whether every feature sym intersects is one it also
// subsets — an intersection its subsetting alone implies (`subobjects subsets objects,
// suboccurrences intersects objects, suboccurrences`), stating no further value.
func (ctx *Context) intersectsOnlySubsetted(sym *symbols.Symbol) bool {
	subsetted := map[*symbols.Symbol]bool{}
	for _, rel := range relationshipsOfKind(sym, ast.RelSubsets) {
		if target := ctx.model.semantics.RelationshipTarget(sym, rel); target != nil {
			subsetted[target] = true
		}
	}
	for _, rel := range relationshipsOfKind(sym, ast.RelIntersects) {
		if target := ctx.model.semantics.RelationshipTarget(sym, rel); target == nil || !subsetted[target] {
			return false
		}
	}
	return true
}
