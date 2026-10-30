package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A snapshot or timeslice usage nested in an occurrence is a portion of it (SysML v2 §7.9.3), the
// same thing occurring (KerML 1.0 §7.4.5): a feature its own body does not value is read from the whole.

// wholeOf is the object inst is a portion of, the portion usage holding inst and the whole's
// feature value of name, when inst is held by a portion usage and the whole holds that feature.
func (ctx *Context) wholeOf(inst *Instance, name string) (whole *Instance, portion *symbols.Symbol, wfv *FeatureValue, ok bool) {
	owner, feature := inst.Owner()
	if owner == nil || feature == "" {
		return nil, nil, nil, false
	}
	holder := owner.FeatureValues[feature]
	if holder == nil || holder.Feature == nil || !isPortionUsage(holder.Feature.Symbol) {
		return nil, nil, nil, false
	}
	wfv, held := owner.FeatureValues[name]
	if !held || wfv.Feature == nil {
		return nil, nil, nil, false
	}
	return owner, holder.Feature.Symbol, wfv, true
}

// isPortionUsage reports a `snapshot` or `timeslice` usage.
func isPortionUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Portion != ast.PortionNone
}

// portionReadsWhole: feat of the portion inst is read from the whole when its own body does not
// value it, its type binds at most a `default`, nothing of it subsets it, and the whole has the feature.
func (ctx *Context) portionReadsWhole(inst *Instance, feat *EffectiveFeature) bool {
	if feat.Symbol == nil || ctx.valueBinds(feat) && !feat.defaultIsFallback() {
		return false
	}
	_, portion, wfv, ok := ctx.wholeOf(inst, feat.Name)
	if !ok || wfv.Feature.Symbol == nil || feat.Symbol.OwnerScope == portion.Scope && !bareRestatement(feat) {
		return false
	}
	if source := ctx.referencedFeatureName(inst, feat.Symbol); source != "" && source != feat.Name {
		return false
	}
	if len(ctx.SubsettingFeatures(nil, inst.Type, feat.Name)) > 0 {
		return false
	}
	return wfv.Feature.Symbol == feat.Symbol || ctx.redefinesTransitively(wfv.Feature.Symbol, feat.Symbol) ||
		ctx.redefinesTransitively(feat.Symbol, wfv.Feature.Symbol) || ctx.redefineInCommon(feat.Symbol, wfv.Feature.Symbol)
}

// bareRestatement reports whether a feature declaration states no value and no body of its
// own (`attribute :>> width;`): restated, it is still read from wherever width is valued.
func bareRestatement(feat *EffectiveFeature) bool {
	usage, ok := feat.Symbol.Decl.(*ast.Usage)
	return ok && usage.Value == nil && len(usage.Members) == 0
}

// redefineInCommon reports whether two features redefine one feature: a portion's bare
// `:>> width;` and its whole's `:>> width = 3` are both the type's width.
func (ctx *Context) redefineInCommon(a, b *symbols.Symbol) bool {
	for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(a) {
		if ctx.redefinesTransitively(b, redefined) {
			return true
		}
	}
	return false
}

// materializeFromWhole holds in fv the whole's value of the feature inst, a portion, does not
// value itself, reading the whole for as long as it holds it. Reports whether it held any.
func (inst *Instance) materializeFromWhole(ctx *Context, fv *FeatureValue, name string) (bool, error) {
	if !ctx.portionReadsWhole(inst, fv.Feature) {
		return false, nil
	}
	whole, _, _, _ := ctx.wholeOf(inst, name)
	wfv, err := whole.GetFeatureValue(ctx, name)
	if err != nil {
		return false, fmt.Errorf("feature %s of the whole of %s: %w", name, inst.Type.Name, err)
	}
	ctx.listRead(wfv, fv)
	val := wfv.HeldValue()
	if ctx.ownsHeld(fv.Feature) {
		val = ctx.partsOfWhole(inst, whole, fv.Feature, val)
	}
	if val.Kind == ValInvalid {
		return false, nil
	}
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return false, err
	}
	if val, err = ctx.holdDeclared(inst, fv, val); err != nil {
		return false, err
	}
	if fv.Feature.Scalar() {
		fv.Value = val
	} else {
		fv.Values = val
	}
	fv.Materialized = true
	return true, nil
}

// partsOfWhole is val, the whole's composite value, as the portion inst holds it: without the whole's portions
// (inst itself, its sibling slices, members a portion usage still has to make) and inst's wholes; nothing when all go.
func (ctx *Context) partsOfWhole(inst, whole *Instance, feat *EffectiveFeature, val Value) Value {
	elements := listedElements(val)
	kept := make([]Value, 0, len(elements))
	for _, el := range elements {
		if id, ok := el.Object(); ok {
			if obj, found := ctx.instances[id]; found && (ctx.heldPortionOf(obj, whole) || ctx.isPortionOf(inst, obj)) {
				continue
			}
		}
		kept = append(kept, el)
	}
	tail := requiredTail(val)
	if tail != nil && ctx.requiredArePortionsOf(whole, tail.required) {
		tail = nil
	} else if len(kept) == len(elements) {
		return val
	}
	if tail != nil {
		return NewSequenceValue(&Sequence{elements: kept, elementUnit: tail.elementUnit, required: tail.required})
	}
	if len(kept) == 0 {
		return Value{}
	}
	if feat.Scalar() {
		return kept[0]
	}
	return ctx.collectionOf(feat, kept)
}

// heldPortionOf reports whether obj is whole itself or a portion of it: held under it through portion usages alone.
func (ctx *Context) heldPortionOf(obj, whole *Instance) bool {
	for obj != nil {
		if obj == whole {
			return true
		}
		owner, feature := obj.Owner()
		if owner == nil {
			return false
		}
		holder := owner.FeatureValues[feature]
		if holder == nil || holder.Feature == nil || !isPortionUsage(holder.Feature.Symbol) {
			return false
		}
		obj = owner
	}
	return false
}

// requiredArePortionsOf reports whether the members r reserves, made when reached, will be portions of whole:
// the feature reserving them is a portion usage of whole or of a portion of it.
func (ctx *Context) requiredArePortionsOf(whole *Instance, r *requiredMembers) bool {
	owner, found := ctx.instances[r.owner]
	if !found {
		return false
	}
	holder := owner.FeatureValues[r.feature]
	return holder != nil && holder.Feature != nil && isPortionUsage(holder.Feature.Symbol) && ctx.heldPortionOf(owner, whole)
}
