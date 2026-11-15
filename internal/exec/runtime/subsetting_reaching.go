package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A subsetted feature need only be accessible to the subsetting one (KerML 1.1 §7.3.4.4): for a context o, S nested
// under the usages U1..Uk of o subsetting `P1.….Pn.T` makes each value of o.U1.….Uk.S one of o.P1.….Pn.T.

// reachingSubsetter is a subsetting feature whose subsetted feature is one of another object.
type reachingSubsetter struct {
	feature *symbols.Symbol
	// down is the usages nested from the context to the owner of feature, outermost first.
	down []*symbols.Symbol
	// up is the features of the chain the target is written as before the subsetted
	// one, outermost first; empty for a target named directly.
	up     []*symbols.Symbol
	target *symbols.Symbol
}

// subsetterSlot is a feature of one object contributing its values to a feature of another.
type subsetterSlot struct {
	holder *Instance
	name   string
}

// reachingSubsetters returns the features declared under typ, itself or what it inherits,
// that subset a feature of another object reached from an object of typ; memoized per type.
func (ctx *Context) reachingSubsetters(typ *symbols.Symbol) []reachingSubsetter {
	if found, ok := ctx.model.reaching[typ]; ok {
		return found
	}
	var found []reachingSubsetter
	collect := func(src *symbols.Symbol) {
		if src != nil && src.Scope != nil {
			found = ctx.collectReachingSubsetters(typ, src.Scope, nil, found)
		}
	}
	collect(typ)
	for _, src := range ctx.model.semantics.MemberSources(typ) {
		collect(src)
	}
	if ctx.model.semantics.MemberSourcesStable(typ) {
		ctx.model.reaching[typ] = found
	}
	return found
}

// collectReachingSubsetters walks the usages nested in scope, down being those entered so far.
func (ctx *Context) collectReachingSubsetters(typ *symbols.Symbol, scope *symbols.Scope, down []*symbols.Symbol, found []reachingSubsetter) []reachingSubsetter {
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if _, ok := sym.Decl.(*ast.Usage); !ok || sym.OwnerScope != scope {
			return true
		}
		for _, rel := range ctx.relationshipsOfKind(sym, ast.RelSubsets) {
			path := ctx.model.semantics.ChainTargetPath(sym, ast.RelSubsets, rel.Target)
			if path == nil {
				if target := ctx.model.semantics.RelationshipTarget(sym, rel); target != nil && !ast.IsFeatureChain(rel.Target) {
					path = []*symbols.Symbol{target}
				}
			}
			if len(path) == 0 || len(path) == 1 && len(down) == 0 || path[len(path)-1] == sym {
				continue
			}
			if !ctx.reachedFrom(typ, down, path[0]) {
				continue
			}
			found = append(found, reachingSubsetter{
				feature: sym,
				down:    down,
				up:      path[:len(path)-1],
				target:  path[len(path)-1],
			})
		}
		if sym.Scope != nil && sym.Scope != scope {
			nested := append(down[:len(down):len(down)], sym)
			found = ctx.collectReachingSubsetters(typ, sym.Scope, nested, found)
		}
		return true
	})
	return found
}

// reachedFrom reports whether first, the feature a subsetting names first, is a feature of
// typ and of none of the usages nested between: the innermost object carrying it is the context.
func (ctx *Context) reachedFrom(typ *symbols.Symbol, down []*symbols.Symbol, first *symbols.Symbol) bool {
	for _, usage := range down {
		if _, ok := ctx.denotedFeature(usage, first); ok {
			return false
		}
	}
	_, ok := ctx.denotedFeature(typ, first)
	return ok
}

// denotes reports whether sym names the feature of inst called name, under any name a redefinition gives it.
func (ctx *Context) denotes(inst *Instance, sym *symbols.Symbol, name string) bool {
	for _, typ := range inst.types() {
		if denoted, ok := ctx.denotedFeature(typ, sym); ok && (denoted == name || ctx.redefinitionAliases(typ, name)[denoted]) {
			return true
		}
	}
	return false
}

// reachingContext is a reaching subsetter with the object its nested usages are read from.
type reachingContext struct {
	context   *Instance
	subsetter *reachingSubsetter
}

// reachingContextsOf returns the subsetters declared elsewhere that subset the named feature
// of inst, each with its context: inst, or the owner holding inst through the chain written.
func (ctx *Context) reachingContextsOf(inst *Instance, name string) []reachingContext {
	var out []reachingContext
	// held is the features the owners hold inst through, innermost first.
	var held []subsetterSlot
	for context := inst; context != nil; {
		for _, typ := range context.types() {
			subsetters := ctx.reachingSubsetters(typ)
			for i := range subsetters {
				sub := &subsetters[i]
				if len(sub.up) != len(held) || !ctx.denotes(inst, sub.target, name) || !ctx.heldThrough(held, sub.up) {
					continue
				}
				if !containsReaching(out, context, sub.feature) {
					out = append(out, reachingContext{context: context, subsetter: sub})
				}
			}
		}
		owner, feature := context.Owner()
		if owner == nil {
			break
		}
		held = append(held, subsetterSlot{holder: owner, name: feature})
		context = owner
	}
	return out
}

func containsReaching(found []reachingContext, context *Instance, feature *symbols.Symbol) bool {
	for _, rc := range found {
		if rc.context == context && rc.subsetter.feature == feature {
			return true
		}
	}
	return false
}

// heldThrough reports whether the chain up, outermost first, names the features held, innermost first.
func (ctx *Context) heldThrough(held []subsetterSlot, up []*symbols.Symbol) bool {
	for i, feature := range up {
		slot := held[len(held)-1-i]
		if !ctx.denotes(slot.holder, feature, slot.name) {
			return false
		}
	}
	return true
}

// reachedByASubsetter reports whether a feature declared elsewhere subsets the named feature of inst.
func (ctx *Context) reachedByASubsetter(inst *Instance, name string) bool {
	return len(ctx.reachingContextsOf(inst, name)) > 0 || len(ctx.namespaceReachingOf(inst, name)) > 0
}

// reachingSlots returns the feature values contributing to the named feature of inst from
// elsewhere, in declaration order, reading the nested usages leading to each; open reads
// them as a model-level read does, following only the objects they certainly hold.
func (ctx *Context) reachingSlots(inst *Instance, name string, open bool) ([]subsetterSlot, error) {
	var slots []subsetterSlot
	for _, rc := range ctx.reachingContextsOf(inst, name) {
		holders := []*Instance{rc.context}
		for _, usage := range rc.subsetter.down {
			var next []*Instance
			for _, holder := range holders {
				objects, err := ctx.objectsHeldAs(holder, usage, open)
				if err != nil {
					return nil, fmt.Errorf("usage %s leading to subsetting feature %s of %s: %w", usage.Name, rc.subsetter.feature.Name, name, err)
				}
				next = append(next, objects...)
			}
			holders = next
		}
		for _, holder := range holders {
			feature, ok := ctx.slotDenoting(holder, rc.subsetter.feature)
			if ok && !(holder == inst && feature == name) && !containsSlot(slots, holder, feature) {
				slots = append(slots, subsetterSlot{holder: holder, name: feature})
			}
		}
	}
	return slots, nil
}

func containsSlot(slots []subsetterSlot, holder *Instance, name string) bool {
	for _, slot := range slots {
		if slot.holder == holder && (slot.name == name || holder.FeatureValues[slot.name] == holder.FeatureValues[name]) {
			return true
		}
	}
	return false
}

// slotDenoting names the feature value of inst that sym or a redefinition of it declares.
func (ctx *Context) slotDenoting(inst *Instance, sym *symbols.Symbol) (string, bool) {
	for _, typ := range inst.types() {
		if name, ok := ctx.denotedFeature(typ, sym); ok {
			if _, held := inst.FeatureValues[name]; held {
				return name, true
			}
		}
	}
	return "", false
}

// objectsHeldAs returns the objects holder holds as the usage sym, each once.
func (ctx *Context) objectsHeldAs(holder *Instance, sym *symbols.Symbol, open bool) ([]*Instance, error) {
	name, ok := ctx.slotDenoting(holder, sym)
	if !ok {
		return nil, nil
	}
	var held Value
	if open {
		fv, population, err := holder.openFeatureValue(ctx, name)
		if err != nil {
			return nil, err
		}
		held = fv.HeldValue()
		if population.Stopped {
			held = sequenceOf(population.Contributed)
		}
	} else {
		fv, err := holder.GetFeatureValue(ctx, name)
		if err != nil {
			return nil, err
		}
		held = fv.HeldValue()
	}
	elements, err := ctx.HeldElements(held)
	if err != nil {
		return nil, err
	}
	var objects []*Instance
	for _, el := range elements {
		if obj, ok := ctx.instances[el.Instance]; ok && el.Kind == ValInstance && obj != nil {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// indexNamespaceReaching records, per namespace usage, the usages declared in a namespace
// or nested under one that subset a feature reached through a chain starting at it
// (`part extra :> rig.members;` beside `part rig`): a namespace usage has no featuring
// type, so every feature can access it.
func (ctx *Context) indexNamespaceReaching(index *namespaceModelIndex, sym *symbols.Symbol, down []*symbols.Symbol) {
	for _, rel := range ctx.relationshipsOfKind(sym, ast.RelSubsets) {
		path := ctx.model.semantics.ChainTargetPath(sym, ast.RelSubsets, rel.Target)
		if len(path) < 2 || path[len(path)-1] == sym {
			continue
		}
		first := ctx.declaredSymbol(path[0])
		if first.OwnerScope == nil || !namespaceScope(first.OwnerScope) || !ctx.reachedFromNamespace(down, path[0]) {
			continue
		}
		index.reaching[first] = append(index.reaching[first], reachingSubsetter{
			feature: sym,
			down:    down,
			up:      path[:len(path)-1],
			target:  path[len(path)-1],
		})
	}
	if sym.Scope == nil {
		return
	}
	nested := append(down[:len(down):len(down)], sym)
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if _, ok := member.Decl.(*ast.Usage); ok && member.OwnerScope == sym.Scope {
			ctx.indexNamespaceReaching(index, member, nested)
		}
		return true
	})
}

// reachedFromNamespace reports whether none of the usages nested between carries first.
func (ctx *Context) reachedFromNamespace(down []*symbols.Symbol, first *symbols.Symbol) bool {
	for _, usage := range down {
		if _, ok := ctx.denotedFeature(usage, first); ok {
			return false
		}
	}
	return true
}

// namespaceReachingOf returns the subsetters declared in a namespace that subset the named
// feature of inst through a chain starting at the namespace usage whose occurrence holds inst.
func (ctx *Context) namespaceReachingOf(inst *Instance, name string) []*reachingSubsetter {
	var held []subsetterSlot
	root := inst
	for {
		owner, feature := root.Owner()
		if owner == nil {
			break
		}
		held = append(held, subsetterSlot{holder: owner, name: feature})
		root = owner
	}
	var out []*reachingSubsetter
	for usage, subsetters := range ctx.namespaceRootUsages(root) {
		for i := range subsetters {
			sub := &subsetters[i]
			if len(sub.up) != len(held)+1 || sub.up[0] != usage || !ctx.denotes(inst, sub.target, name) || !ctx.heldThrough(held, sub.up[1:]) {
				continue
			}
			out = append(out, sub)
		}
	}
	slices.SortStableFunc(out, func(a, b *reachingSubsetter) int {
		switch {
		case declaredBefore(a.feature, b.feature):
			return -1
		case declaredBefore(b.feature, a.feature):
			return 1
		}
		return 0
	})
	return out
}

// namespaceRootUsages is the namespace usages root is an occurrence of that a subsetter
// reaches through, each with those subsetters.
func (ctx *Context) namespaceRootUsages(root *Instance) map[*symbols.Symbol][]reachingSubsetter {
	reaching := ctx.namespaceModelIndex().reaching
	if len(reaching) == 0 {
		return nil
	}
	var out map[*symbols.Symbol][]reachingSubsetter
	for usage, subsetters := range reaching {
		if slices.Contains(ctx.occurrences[usage], root.ID) {
			if out == nil {
				out = make(map[*symbols.Symbol][]reachingSubsetter)
			}
			out[usage] = subsetters
		}
	}
	return out
}

// namespaceReachingContributions appends to values what the subsetters declared in a namespace
// contribute to the named feature of inst: the objects such a usage denotes, or those the
// usages nested under one hold.
func (ctx *Context) namespaceReachingContributions(inst *Instance, name string, open bool, values []Value, seen map[int64]bool) ([]Value, map[int64]bool, error) {
	for _, sub := range ctx.namespaceReachingOf(inst, name) {
		top := sub.feature
		if len(sub.down) > 0 {
			top = sub.down[0]
		}
		holders, err := ctx.denotedSubsetObjects(ctx.declaredSymbol(top))
		if err != nil {
			return nil, nil, fmt.Errorf("subsetting usage %s of %s: %w", SymbolText(sub.feature), name, err)
		}
		if len(sub.down) == 0 {
			for _, obj := range holders {
				values, seen = appendUniqueInstances(values, []Value{{Kind: ValInstance, Instance: obj.ID}}, seen)
			}
			continue
		}
		for _, usage := range sub.down[1:] {
			var next []*Instance
			for _, holder := range holders {
				objects, err := ctx.objectsHeldAs(holder, usage, open)
				if err != nil {
					return nil, nil, fmt.Errorf("usage %s leading to subsetting feature %s of %s: %w", usage.Name, sub.feature.Name, name, err)
				}
				next = append(next, objects...)
			}
			holders = next
		}
		for _, holder := range holders {
			feature, ok := ctx.slotDenoting(holder, sub.feature)
			if !ok || holder == inst && feature == name {
				continue
			}
			objects, err := ctx.objectsHeldAs(holder, sub.feature, open)
			if err != nil {
				return nil, nil, fmt.Errorf("subsetting feature %s of %s: %w", feature, name, err)
			}
			for _, obj := range objects {
				values, seen = appendUniqueInstances(values, []Value{{Kind: ValInstance, Instance: obj.ID}}, seen)
			}
		}
	}
	return values, seen, nil
}

// reachedSubsetted returns the features of other objects that the named feature of inst
// subsets by a declaration elsewhere: the inverse of reachingSlots, read from the context
// down the chain the target is written as.
func (ctx *Context) reachedSubsetted(inst *Instance, name string) ([]subsetterSlot, error) {
	var slots []subsetterSlot
	// held is the features the owners hold inst through, innermost first.
	var held []subsetterSlot
	for context := inst; context != nil; {
		for _, typ := range context.types() {
			subsetters := ctx.reachingSubsetters(typ)
			for i := range subsetters {
				sub := &subsetters[i]
				if len(sub.down) != len(held) || !ctx.denotes(inst, sub.feature, name) || !ctx.heldThrough(held, sub.down) {
					continue
				}
				targets := []*Instance{context}
				for _, feature := range sub.up {
					var next []*Instance
					for _, target := range targets {
						objects, err := ctx.objectsHeldAs(target, feature, false)
						if err != nil {
							return nil, fmt.Errorf("feature %s leading to subsetted feature %s of %s: %w", feature.Name, sub.target.Name, name, err)
						}
						next = append(next, objects...)
					}
					targets = next
				}
				for _, target := range targets {
					feature, ok := ctx.slotDenoting(target, sub.target)
					if ok && !(target == inst && feature == name) && !containsSlot(slots, target, feature) {
						slots = append(slots, subsetterSlot{holder: target, name: feature})
					}
				}
			}
		}
		owner, feature := context.Owner()
		if owner == nil {
			break
		}
		held = append(held, subsetterSlot{holder: owner, name: feature})
		context = owner
	}
	return slots, nil
}

// subsetsElsewhere reports whether a declaration makes the named feature of inst subset a
// feature of another object, without reading the chain to it.
func (ctx *Context) subsetsElsewhere(inst *Instance, name string) bool {
	var held []subsetterSlot
	for context := inst; context != nil; {
		for _, typ := range context.types() {
			subsetters := ctx.reachingSubsetters(typ)
			for i := range subsetters {
				sub := &subsetters[i]
				if len(sub.down) == len(held) && ctx.denotes(inst, sub.feature, name) && ctx.heldThrough(held, sub.down) {
					return true
				}
			}
		}
		owner, feature := context.Owner()
		if owner == nil {
			break
		}
		held = append(held, subsetterSlot{holder: owner, name: feature})
		context = owner
	}
	return false
}

// heldDown returns the objects already held from context down the usages given, reading nothing.
func (ctx *Context) heldDown(context *Instance, down []*symbols.Symbol) []*Instance {
	holders := []*Instance{context}
	for _, usage := range down {
		var next []*Instance
		for _, holder := range holders {
			name, ok := ctx.slotDenoting(holder, usage)
			if !ok || !holder.FeatureValues[name].Materialized {
				continue
			}
			for _, el := range elementsOf(holder.FeatureValues[name].HeldValue()) {
				if obj := ctx.instances[el.Instance]; el.Kind == ValInstance && obj != nil {
					next = append(next, obj)
				}
			}
		}
		holders = next
	}
	return holders
}
