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
func (ctx *Context) kernelCollectionRead(inst *Instance, name string) (*FeatureValue, bool, error) {
	sym, ok := ctx.kernelCollection(inst.Type, name)
	if !ok {
		return nil, false, nil
	}
	fv, err := ctx.kernelCollectionValue(inst, sym)
	return fv, true, err
}

// kernelCollection is the Kernel-tier collection feature of typ that name reads: a
// frame-tier usage with an upper bound above one that is none of the frame roots the
// runtime answers itself.
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
	upper := ctx.featureMultiplicity(sym, typ).Upper
	if !upper.Infinite && (!upper.Known || upper.Value <= 1) {
		return nil, false
	}
	return sym, true
}

// kernelCollectionValue is the collection sym holds on inst: the elements of every
// shape feature subsetting it, each object once unless sym is nonunique. It is
// derived on each read and stored nowhere, so the shape stays as declared.
func (ctx *Context) kernelCollectionValue(inst *Instance, sym *symbols.Symbol) (*FeatureValue, error) {
	feat := ctx.effectiveFeature(sym.Name, sym, inst.Type)
	var values []Value
	var seen map[int64]bool
	features := ctx.FeaturesOf(inst.Type)
	for i := range features {
		shape := &features[i]
		if shape.Symbol == nil || !ctx.subsetsTransitively(shape.Symbol, inst.Type, sym) {
			continue
		}
		fv, err := inst.GetFeatureValue(ctx, shape.Name)
		if err != nil {
			return nil, err
		}
		held, err := ctx.HeldElements(fv.HeldValue())
		if err != nil {
			return nil, err
		}
		if feat.Unique {
			values, seen = appendUniqueInstances(values, held, seen)
		} else {
			values = append(values, held...)
		}
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, err
	}
	return &FeatureValue{Feature: &feat, Values: ctx.collectionOf(&feat, values), Materialized: true}, nil
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
