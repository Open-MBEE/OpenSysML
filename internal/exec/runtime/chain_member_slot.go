package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A feature value belongs to a feature, not to a name: a redefinition makes two
// names read one value (KerML 1.0 §7.3.4.5), while two features that happen to
// share a name on one type — a namesake inherited along another path beside a
// redefined one — keep values of their own. A chain's segment therefore reads
// the feature it resolves to in the namespace of the segment before it, and
// the object answers with the slot holding that feature's value.

// noteChainMembers records the declaration each segment of a chain names, in
// the namespace of the segment before it, for chainObjectMember to read by.
func (ec *EvalContext) noteChainMembers(base ast.Node, parts []ast.NameSegment) {
	cur, ok := ec.chainBaseSymbol(base)
	if !ok {
		return
	}
	for i := range parts {
		decl, found := ec.ctx.declaredMember(cur, parts[i].Text)
		if !found || decl == nil {
			return
		}
		if ec.chainMembers == nil {
			ec.chainMembers = make(map[*ast.NameSegment]*symbols.Symbol)
		}
		ec.chainMembers[&parts[i]] = decl
		cur = decl
	}
}

// featureSlot is the name inst holds feature's value under: the feature's own
// slot, or that of a feature of inst's types redefining it, whose value a
// redefined feature shares. False when inst holds no value for it.
func (ctx *Context) featureSlot(inst *Instance, feature *symbols.Symbol) (string, bool) {
	if feature == nil {
		return "", false
	}
	for _, typ := range inst.types() {
		slot := ctx.slotOf(typ, feature)
		if _, held := inst.FeatureValues[slot]; slot != "" && held {
			return slot, true
		}
	}
	return "", false
}

// slotOf is the name typ's objects hold feature's value under, "" when none;
// memoized per type and feature.
func (ctx *Context) slotOf(typ, feature *symbols.Symbol) string {
	key := featureOfType{feature: feature, owner: typ}
	if slot, ok := ctx.model.slots[key]; ok {
		return slot
	}
	slot := ""
	features := ctx.FeaturesOf(typ)
	if i := slices.IndexFunc(features, func(f EffectiveFeature) bool { return f.Symbol == feature }); i >= 0 {
		slot = features[i].Name
	} else if i := slices.IndexFunc(features, func(f EffectiveFeature) bool {
		return f.Symbol != nil && slices.Contains(ctx.redefinedFeatures(f.Symbol, typ), feature)
	}); i >= 0 {
		slot = features[i].Name
	}
	ctx.model.slots[key] = slot
	return slot
}
