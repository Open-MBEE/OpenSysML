package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kernelCollectionRead answers a read of a Kernel collection the shape leaves out from the
// shape features that subset it (KerML 1.0 §7.3.4.4); the second result is whether name is one.
func (ctx *Context) kernelCollectionRead(inst *Instance, name string, open *openPopulation) (*FeatureValue, bool, error) {
	sym, ok := ctx.kernelCollection(inst.Type, name)
	if !ok {
		return nil, false, nil
	}
	fv, err := ctx.kernelCollectionValue(inst, sym, open)
	return fv, true, err
}

// kernelCollection is the Kernel-tier `[0..*]` collection of typ that name reads, if the library
// defines it by subsetting alone (no `inverse of`, `unions`, own value, lower bound or frame root).
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

// kernelCollectionValue unions, in shape order, what the shape features subsetting sym hold on
// inst (each object once unless nonunique); an open subsetter on a model-level read leaves it open.
func (ctx *Context) kernelCollectionValue(inst *Instance, sym *symbols.Symbol, open *openPopulation) (*FeatureValue, error) {
	feat := ctx.effectiveFeature(sym.Name, sym, inst.Type)
	values, stopped, err := ctx.shapeContributions(inst, sym, &feat, nil, open)
	if err != nil {
		return nil, err
	}
	return &FeatureValue{Feature: &feat, Values: ctx.collectionOf(&feat, values), Materialized: !stopped}, nil
}

// shapeContributions unions onto held, in shape order, what the shape features of inst
// subsetting the Kernel collection sym hold (KerML 1.0 §7.3.4.4), each object once unless
// feat is nonunique. A non-nil open records an open subsetter on a model-level read instead
// of making up its lower bound, with the values before it as the positions the model fixes;
// the second result reports that it did.
func (ctx *Context) shapeContributions(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature, held []Value, open *openPopulation) ([]Value, bool, error) {
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
	add(held)
	fixed := -1
	for _, shape := range ctx.shapeSubsettersOf(inst, sym) {
		var sub *openPopulation
		if open != nil {
			sub = &openPopulation{}
		}
		fv, err := inst.getFeatureValue(ctx, shape.Name, sub)
		if err != nil {
			return nil, false, err
		}
		if sub != nil && sub.Stopped {
			if !stopped {
				stopped, fixed = true, len(values)
			}
			atLeast = max(atLeast, int64(len(sub.Contributed)), sub.AtLeast, fewestOf(shape.Multiplicity))
			add(sub.Contributed)
			continue
		}
		held, err := ctx.HeldElements(fv.HeldValue())
		if err != nil {
			return nil, false, err
		}
		add(held)
	}
	if stopped {
		open.Stopped, open.Contributed, open.AtLeast, open.Fixed = true, values, atLeast, fixed
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, false, err
	}
	return values, stopped, nil
}

// shapeSubsettersOf lists the shape features of inst, through every type classifying it,
// that transitively subset the Kernel collection sym, in shape order and once per name.
func (ctx *Context) shapeSubsettersOf(inst *Instance, sym *symbols.Symbol) []*EffectiveFeature {
	var out []*EffectiveFeature
	var seen map[string]bool
	types := inst.types()
	if len(types) > 1 {
		seen = make(map[string]bool)
	}
	for _, typ := range types {
		features := ctx.FeaturesOf(typ)
		for i := range features {
			shape := &features[i]
			if shape.Symbol == nil || seen[shape.Name] || !ctx.subsetsTransitively(shape.Symbol, typ, sym) {
				continue
			}
			if seen != nil {
				seen[shape.Name] = true
			}
			out = append(out, shape)
		}
	}
	return out
}

// subsetsTransitively reports whether feature, a member of owner, subsets target through
// any chain of declared `:>` edges, implicit nesting subsettings and redefinitions.
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

// intersectsOnlySubsetted reports whether every feature sym intersects is one it also subsets,
// an intersection its subsetting alone implies (`subobjects`), so it states no further value.
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
