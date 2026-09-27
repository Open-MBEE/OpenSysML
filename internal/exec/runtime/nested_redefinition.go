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
		overrides[p.rest[0]] = p.sym
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
