package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A snapshot or timeslice usage nested in an occurrence is a portion of that occurrence
// (SysML v2 §7.9.3: it subsets its owner's `snapshots`/`timeSlices`, `Occurrences.kerml`
// `portion feature`), the same thing occurring with the same identity (KerML 1.0 §7.4.5), so
// a feature the portion's own body does not value is read from the whole.

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

// portionReadsWhole reports whether feat of inst, a portion, is read from the whole: the
// portion's own body does not value it (a bare `:>> width;` restates it, no more), its type binds
// it to nothing or a `default` (a binding the type states, `product = width * height`, holds over
// the portion's own values), nothing of the portion subsets it, and the whole's feature of the
// name is the same feature or a redefinition on either side.
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
	return ok && feat.DefaultValue == nil && len(usage.Members) == 0
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
