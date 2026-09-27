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
// names feature (see semantics.NestedRedefinitionsOf).
func (ctx *Context) pendingNestedRedefinitions(owner *Instance, feature string) []pendingRedefinition {
	if owner == nil || feature == "" {
		return nil
	}
	// A redefined member shares one feature value under every name it reads
	// as, so a chain naming any of them applies to the member materialized here.
	names := map[string]bool{feature: true}
	if fv := owner.FeatureValues[feature]; fv != nil {
		for name, other := range owner.FeatureValues {
			if other == fv {
				names[name] = true
			}
		}
	}
	var out []pendingRedefinition
	for _, p := range owner.nested {
		if len(p.rest) > 0 && names[p.rest[0]] {
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
				if len(nr.Path) > 1 && names[nr.Path[0]] {
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
		taken, ok := overrides[p.rest[0]]
		if !ok {
			overrides[p.rest[0]] = p.sym
			continue
		}
		// A chain declared by a type specializing the earlier chain's context
		// outranks it, as a nested redefining body in the subtype does; equal
		// and unrelated contexts keep the first, the tails carried down first.
		if ctx.chainOutranks(p.sym, taken) {
			overrides[p.rest[0]] = p.sym
		}
	}
	cloned := false
	for _, p := range carry {
		for i := range features {
			if features[i].Name == p.rest[0] && features[i].DefaultValue != nil && valuedChain(p) && ctx.chainGovernsValue(p.sym, features[i].Symbol) {
				if !cloned {
					features = slices.Clone(features)
					cloned = true
				}
				features[i].GovernedByChain = true
			}
		}
	}
	if len(overrides) > 0 {
		if !cloned {
			features = slices.Clone(features)
		}
		for i := range features {
			redefining, ok := overrides[features[i].Name]
			if !ok {
				continue
			}
			// A redefinition declared by the chain's context or a type
			// specializing it wins over the chain, as the nested-body form does.
			if hasRedefines(features[i].Symbol) && ctx.blocksChain(features[i].Symbol, redefining) {
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
	// A valued chain reaching a bound member governs its inherited binding as a
	// redefining body does when the chain's context specializes the binding's:
	// mark the feature and reinstall it so the next read materializes a fresh
	// object the chain applies below, not the bound one.
	if len(chain) > 1 && fv.Feature != nil && fv.Feature.DefaultValue != nil && !fv.Feature.GovernedByChain &&
		valuedChain(pendingRedefinition{rest: chain, sym: sym}) && ctx.chainGovernsValue(sym, fv.Feature.Symbol) {
		feat := *fv.Feature
		feat.GovernedByChain = true
		feat.DefaultValue = nil
		return ctx.installFeatureValue(inst, fv, &feat)
	}
	for _, el := range elementsOf(fv.HeldValue()) {
		id, ok := el.Object()
		if !ok {
			continue
		}
		child, ok := ctx.instances[id]
		// A reference, port or subject holds an object it does not own, and a
		// sibling feature can hold an object inst owns through another feature:
		// the chain stays within objects inst owns through this feature value.
		if !ok || child.owner != inst || inst.FeatureValues[child.ownerFeature] != fv {
			continue
		}
		if err := ctx.refineChildBelow(inst, child, chain[1:], sym); err != nil {
			return err
		}
	}
	return nil
}

// refineChildBelow applies the rest of a chain reaching below inst's child
// object: a one-segment rest refines that feature of it, a longer one is
// carried on the child and reaches the grandchildren it already materialized.
func (ctx *Context) refineChildBelow(inst, child *Instance, rest []string, sym *symbols.Symbol) error {
	if len(rest) == 1 {
		cfv := child.FeatureValues[rest[0]]
		if cfv == nil || cfv.Feature == nil {
			return nil
		}
		// A redefinition declared by the chain's context or a type
		// specializing it wins over the chain, and a chain host installed by an
		// unrelated classifier keeps its place, as the lazy path's first-wins
		// rule keeps it; a plain feature is no redefinition and yields.
		if hasRedefines(cfv.Feature.Symbol) && (ctx.blocksChain(cfv.Feature.Symbol, sym) ||
			(isChainHost(cfv.Feature.Symbol) && !ctx.chainOutranks(sym, cfv.Feature.Symbol))) {
			return nil
		}
		feat := ctx.effectiveFeature(rest[0], sym, child.Type)
		return ctx.installFeatureValue(child, cfv, &feat)
	}
	ctx.noteProbeUndo(func() { child.nested = child.nested[:len(child.nested)-1] })
	child.nested = append(child.nested, pendingRedefinition{rest: rest, sym: sym})
	return ctx.refineNestedBelow(child, rest, sym)
}

// applyPendingToHeld applies the nested redefinitions a type of inst declares
// below fv's feature to the objects val holds: a composite feature owns the
// objects it holds however they arrived, so a bound or written object reads
// the chain a redefining body would give it. A governed feature materialized
// a fresh object the chain already rode down.
func (ctx *Context) applyPendingToHeld(inst *Instance, fv *FeatureValue, val Value) error {
	if fv.Feature == nil || fv.Feature.GovernedByChain || !ctx.ownsHeld(fv.Feature) {
		return nil
	}
	for _, p := range ctx.pendingNestedRedefinitions(inst, fv.Feature.Name) {
		if len(p.rest) == 0 {
			continue
		}
		for _, el := range elementsOf(val) {
			id, ok := el.Object()
			if !ok {
				continue
			}
			child, ok := ctx.instances[id]
			if !ok {
				continue
			}
			if err := ctx.refineChildBelow(inst, child, p.rest, p.sym); err != nil {
				return err
			}
		}
	}
	return nil
}

// clonePendingRedefinitions copies the pending tails of a nested redefinition by value, for
// an image to hold and a materialization to restore.
func clonePendingRedefinitions(pending []pendingRedefinition) []pendingRedefinition {
	out := slices.Clone(pending)
	for i := range out {
		out[i].rest = slices.Clone(out[i].rest)
	}
	return out
}

// redefinitionContext answers the type or usage whose body member's
// redefinition is written in: the first definition up the owner chain, or the
// topmost usage when no definition encloses it (a chain declared on
// `part top : Derived { attribute :>> mid.leaf.value = 99.0; }` counts as
// declared by top).
func (ctx *Context) redefinitionContext(member *symbols.Symbol) *symbols.Symbol {
	owner := ctx.findOwnerType(member)
	for owner != nil && !isDefinitionSymbol(owner) {
		next := ctx.findOwnerType(owner)
		if next == nil || (!isDefinitionSymbol(next) && !isUsageSymbol(next)) {
			break
		}
		owner = next
	}
	return owner
}

// isUsageSymbol reports whether sym is declared by a usage.
func isUsageSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	_, ok := sym.Decl.(*ast.Usage)
	return ok
}

// isChainHost reports whether member redefines a feature chain — it is the
// host feature a chain redefinition parses to.
func isChainHost(member *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(member) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			if _, ok := rel.Target.(*ast.FeatureChainExpr); ok {
				return true
			}
		}
	}
	return false
}

// chainOutranks reports whether the chain next was declared by a context
// strictly specializing the chain prior's: the newer chain then replaces it,
// as a nested redefining body in the subtype does.
func (ctx *Context) chainOutranks(next, prior *symbols.Symbol) bool {
	nextCtx, priorCtx := ctx.redefinitionContext(next), ctx.redefinitionContext(prior)
	return nextCtx != priorCtx && ctx.modelConforms(nextCtx, priorCtx)
}

// valuedChain reports whether a pending chain member states a value: its own,
// or one its body states at any depth. A chain declaring only a type or
// multiplicity conflicts with nothing.
func valuedChain(p pendingRedefinition) bool {
	usage, ok := p.sym.Decl.(*ast.Usage)
	return ok && valuesAFeature(usage)
}

// chainGovernsValue reports whether a chain's context strictly specializes the
// context the valued feature is declared in — the more specific body governs
// the inherited binding, as a redefining body does.
func (ctx *Context) chainGovernsValue(chain, valued *symbols.Symbol) bool {
	chainCtx, valuedCtx := ctx.redefinitionContext(chain), ctx.redefinitionContext(valued)
	return chainCtx != valuedCtx && ctx.modelConforms(chainCtx, valuedCtx)
}

// blocksChain reports whether existing, the redefinition standing on a
// feature, outranks a chain reaching it: it does when the body declaring it
// conforms to the chain's context — the same body, or a subtype of it.
func (ctx *Context) blocksChain(existing, chain *symbols.Symbol) bool {
	return ctx.modelConforms(ctx.redefinitionContext(existing), ctx.redefinitionContext(chain))
}

// hasRedefines reports whether member redefines another feature.
func hasRedefines(member *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(member) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}
