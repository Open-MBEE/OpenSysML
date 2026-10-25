package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A feature value belongs to a feature, not to a name: a redefinition makes two
// names read one value (KerML 1.0 §7.3.4.5), while two features that happen to
// share a name on one type — a namesake inherited along another path beside a
// redefined one — keep values of their own. A read or write therefore resolves
// its name to a feature first — a chain's segment in the namespace of the segment
// before it, a bare name in the scope the expression was written in — and the
// object answers with the slot holding that feature's value.

// chainStatic is the declaration each segment of the chain being evaluated
// names; decls[i] is for parts[i], nil past the first segment left unresolved.
type chainStatic struct {
	parts []ast.NameSegment
	decls []*symbols.Symbol
}

// noteChainMembers records the declarations a chain's segments name for the
// reads of its parts, and returns the restore of what was recorded before.
func (ec *EvalContext) noteChainMembers(base ast.Node, parts []ast.NameSegment) func() {
	previous := ec.chain
	ec.chain = chainStatic{parts: parts, decls: ec.chainDeclarations(base, parts)}
	return func() { ec.chain = previous }
}

// chainDeclarations resolves each segment in the namespace of the one before
// it, from the feature or cast type the base names; nil when the base names none.
func (ec *EvalContext) chainDeclarations(base ast.Node, parts []ast.NameSegment) []*symbols.Symbol {
	cur, ok := ec.chainBaseSymbol(base)
	if !ok {
		return nil
	}
	decls := make([]*symbols.Symbol, len(parts))
	for i := range parts {
		decl, found := ec.ctx.declaredMember(cur, parts[i].Text)
		if !found || decl == nil {
			break
		}
		decls[i] = decl
		cur = decl
	}
	return decls
}

// chainMember is the declaration the first of parts names, when parts is a tail
// of the chain being evaluated; nil otherwise.
func (ec *EvalContext) chainMember(parts []ast.NameSegment) *symbols.Symbol {
	whole := ec.chain.parts
	if len(parts) == 0 || len(parts) > len(whole) || len(ec.chain.decls) != len(whole) {
		return nil
	}
	if &parts[len(parts)-1] != &whole[len(whole)-1] {
		return nil
	}
	return ec.chain.decls[len(whole)-len(parts)]
}

// chainTargetMember is the declaration an assignment target's feature names, in
// the namespace the target's steps reach from its base; nil when unresolved.
func (ec *EvalContext) chainTargetMember(target *lower.AssignTarget, feature string) *symbols.Symbol {
	cur, ok := ec.chainBaseSymbol(target.Base)
	if !ok {
		return nil
	}
	for _, step := range append(slices.Clone(target.Steps), feature) {
		decl, found := ec.ctx.declaredMember(cur, step)
		if !found || decl == nil {
			return nil
		}
		cur = decl
	}
	return cur
}

// selfMemberSlot is the slot of the bound object holding the feature name
// resolves to in the evaluation's scope, when the object holds it under another
// name; name itself otherwise.
func (ec *EvalContext) selfMemberSlot(name string) string {
	if sym, ok := ec.lookupName(name); ok && sym != nil {
		if slot, held := ec.ctx.featureSlot(ec.self, sym); held {
			return slot
		}
	}
	return name
}

// featureSlot is the name inst holds feature's value under: the feature's own
// slot, or that of a feature of inst's types redefining it, whose value a
// redefined feature shares. False when inst holds no value for it.
func (ctx *Context) featureSlot(inst *Instance, feature *symbols.Symbol) (string, bool) {
	if feature == nil || inst == nil {
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
