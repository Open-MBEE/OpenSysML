package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// pendingRedefinition is the tail of a nested redefinition still to apply below
// an object: rest is the chain under the member it materializes, sym the
// redefining member the last segment's feature takes.
type pendingRedefinition struct {
	rest []string
	sym  *symbols.Symbol
}

// pendingNestedRedefinitions returns the nested redefinitions applying to the
// object materialized as the member feature of owner: the tails carried down
// from above, plus the chains every type of owner declares whose first segment
// names feature (an OpenSysML extension to SysML v2; see
// semantics.NestedRedefinitionsOf).
func (ctx *Context) pendingNestedRedefinitions(owner *Instance, feature string) []pendingRedefinition {
	if owner == nil || feature == "" {
		return nil
	}
	var out []pendingRedefinition
	for _, p := range owner.nested {
		if len(p.rest) > 0 && p.rest[0] == feature {
			out = append(out, pendingRedefinition{rest: p.rest[1:], sym: p.sym})
		}
	}
	seen := make(map[*symbols.Symbol]bool)
	for _, typ := range owner.types() {
		for _, src := range append([]*symbols.Symbol{typ}, ctx.model.semantics.MemberSources(typ)...) {
			if src == nil || seen[src] {
				continue
			}
			seen[src] = true
			for _, nr := range ctx.model.semantics.NestedRedefinitionsOf(src) {
				if len(nr.Path) > 1 && nr.Path[0] == feature {
					out = append(out, pendingRedefinition{rest: nr.Path[1:], sym: nr.Feature})
				}
			}
		}
	}
	return out
}

// applyNestedRedefinitions applies the pending redefinitions to the shape of
// the object being materialized as sym: a one-segment rest redefines that
// feature of the object here, a longer one is carried on the object for its
// members. It returns the effective features to use and the tails to carry.
func (ctx *Context) applyNestedRedefinitions(sym *symbols.Symbol, features []EffectiveFeature, pending []pendingRedefinition) ([]EffectiveFeature, []pendingRedefinition) {
	var carry []pendingRedefinition
	var overrides map[string]*symbols.Symbol
	for _, p := range pending {
		if len(p.rest) == 0 {
			continue
		}
		if len(p.rest) > 1 {
			carry = append(carry, p)
			continue
		}
		if overrides == nil {
			overrides = make(map[string]*symbols.Symbol)
		}
		// The first chain reaching a feature wins: the tails carried down are
		// listed first, then each type's own before its member sources'.
		if _, taken := overrides[p.rest[0]]; !taken {
			overrides[p.rest[0]] = p.sym
		}
	}
	if len(overrides) > 0 {
		features = slices.Clone(features)
		for i := range features {
			redefining, ok := overrides[features[i].Name]
			if !ok {
				continue
			}
			// A standard redefinition the object's own type declares wins over a
			// chain reaching the same feature from above.
			if ctx.ownBodyRedefinition(sym, features[i].Symbol) {
				continue
			}
			features[i] = ctx.effectiveFeature(features[i].Name, redefining, sym)
		}
	}
	return features, carry
}

// applyClassifierNestedRedefinitions applies the nested redefinitions typ and
// its member sources declare to the children inst already holds, as a carried
// direct feature's redefinition refines one (see classify).
func (ctx *Context) applyClassifierNestedRedefinitions(inst *Instance, typ *symbols.Symbol) error {
	for _, src := range append([]*symbols.Symbol{typ}, ctx.model.semantics.MemberSources(typ)...) {
		for _, nr := range ctx.model.semantics.NestedRedefinitionsOf(src) {
			if len(nr.Path) < 2 {
				continue
			}
			if err := ctx.refineNestedBelow(inst, nr.Path, nr.Feature); err != nil {
				return err
			}
		}
	}
	return nil
}

// refineNestedBelow walks chain below inst: its last segment refines the
// feature value of every child the chain reaches, and a longer rest carries on
// each child and reaches the grandchildren it already materialized.
func (ctx *Context) refineNestedBelow(inst *Instance, chain []string, sym *symbols.Symbol) error {
	fv := inst.FeatureValues[chain[0]]
	if fv == nil {
		return nil
	}
	for _, el := range elementsOf(fv.HeldValue()) {
		id, ok := el.Object()
		if !ok {
			continue
		}
		child, ok := ctx.instances[id]
		if !ok {
			continue
		}
		rest := chain[1:]
		if len(rest) == 1 {
			cfv := child.FeatureValues[rest[0]]
			if cfv == nil || cfv.Feature == nil {
				continue
			}
			feat := ctx.effectiveFeature(rest[0], sym, child.Type)
			if err := ctx.refineFeatureValue(child, cfv, &feat, child.Type); err != nil {
				return err
			}
			continue
		}
		ctx.noteProbeUndo(func() { child.nested = child.nested[:len(child.nested)-1] })
		child.nested = append(child.nested, pendingRedefinition{rest: rest, sym: sym})
		if err := ctx.refineNestedBelow(child, rest, sym); err != nil {
			return err
		}
	}
	return nil
}

// clonePending copies the pending tails of a nested redefinition by value, for
// an image to hold and a materialization to restore.
func clonePendingRedefinitions(pending []pendingRedefinition) []pendingRedefinition {
	out := slices.Clone(pending)
	for i := range out {
		out[i].rest = slices.Clone(out[i].rest)
	}
	return out
}

// ownBodyRedefinition reports whether member is a redefinition declared by
// sym's own body, which a nested redefinition reaching the same feature does
// not override.
func (ctx *Context) ownBodyRedefinition(sym, member *symbols.Symbol) bool {
	if member == nil || ctx.findOwnerType(member) != sym {
		return false
	}
	for _, rel := range semantics.RelationshipsOf(member) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}
