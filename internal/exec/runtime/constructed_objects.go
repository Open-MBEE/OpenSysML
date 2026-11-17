package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// checkObjects are the objects a check under way made for itself, which end with
// it: the exemplars `this` denotes and the objects its expressions construct.
type checkObjects struct {
	outer *checkObjects
	mark  int
	// actions and behaviors are the action invocations and behavior starts under way
	// when the check began: a body run under it keeps what it constructs in its own frames.
	actions, behaviors int
	// noObject marks a check about no object, which reads a type's declared defaults:
	// `this` there is one exemplar of the type, made when first read.
	noObject    bool
	exemplars   map[*symbols.Symbol]*Instance
	constructed []*Instance
}

// beginCheckObjects opens a check, about no object when noObject, and returns what
// closes it, abandoning the objects the check made that nothing outliving it holds.
func (ctx *Context) beginCheckObjects(noObject bool) func() {
	set := &checkObjects{
		outer: ctx.checkObjects, mark: len(ctx.created),
		actions: ctx.actionDepth, behaviors: ctx.behaviorRunDepth, noObject: noObject,
	}
	ctx.checkObjects = set
	return func() {
		ctx.checkObjects = set.outer
		ctx.releaseCheckObjects(set)
	}
}

// noteConstructed records inst as constructed by the check under way, if any, whose
// own evaluation made it rather than a body an action or a behavior runs under it.
func (ctx *Context) noteConstructed(inst *Instance) {
	set := ctx.checkObjects
	if set == nil || ctx.actionDepth != set.actions || ctx.behaviorRunDepth != set.behaviors {
		return
	}
	set.constructed = append(set.constructed, inst)
}

// releaseCheckObjects abandons what the closed check made, with what those objects hold: its
// exemplars, and the objects it constructed that nothing outliving it names, the rest passing outward.
func (ctx *Context) releaseCheckObjects(set *checkObjects) {
	roots := make(map[*Instance]bool, len(set.exemplars)+len(set.constructed))
	for _, inst := range set.exemplars {
		roots[inst] = true
	}
	constructed := make(map[*Instance]bool, len(set.constructed))
	for _, inst := range set.constructed {
		if ctx.instances[inst.ID] == inst && inst.owner == nil && !inst.explicit {
			constructed[inst], roots[inst] = true, true
		}
	}
	if len(roots) == 0 {
		return
	}
	held := ctx.heldSince(set.mark, roots)
	for len(constructed) > 0 {
		kept := false
		for _, id := range ctx.namedOutside(held) {
			root := ctx.instances[id]
			for root.owner != nil && held[root.owner.ID] {
				root = root.owner
			}
			if constructed[root] {
				delete(constructed, root)
				delete(roots, root)
				kept = true
				if set.outer != nil {
					set.outer.constructed = append(set.outer.constructed, root)
				}
			}
		}
		if !kept {
			break
		}
		held = ctx.heldSince(set.mark, roots)
	}
	if len(roots) == 0 {
		return
	}
	var behaviors []*ObjectBehavior
	for _, behavior := range ctx.objectBehaviors {
		if held[behavior.Object.ID] {
			behaviors = append(behaviors, behavior)
		}
	}
	ctx.forgetBehaviors(behaviors)
	ctx.forgetAbandoned(ctx.dropInstances(set.mark, len(ctx.created), func(inst *Instance) bool {
		return held[inst.ID]
	}))
}

// namedOutside lists the objects among set that something outside it names: a value or a
// connector end of an object not in set, a namespace binding, a shared default, a queued message.
func (ctx *Context) namedOutside(set map[int64]bool) []int64 {
	var named []int64
	seen := make(map[int64]bool)
	note := func(val Value) {
		eachObjectNamed(val, func(id int64) {
			if set[id] && !seen[id] {
				seen[id] = true
				named = append(named, id)
			}
		})
	}
	for id, inst := range ctx.instances {
		if set[id] {
			continue
		}
		for _, fv := range inst.FeatureValues {
			note(fv.Value)
			note(fv.Values)
		}
		for _, end := range inst.Ends {
			note(end.Value)
		}
	}
	for _, val := range ctx.namespaceBindings {
		note(val)
	}
	for _, shared := range ctx.sharedDefaults {
		note(shared.value)
	}
	for _, msg := range ctx.messages {
		if msg.Value != nil {
			note(*msg.Value)
		}
		for _, val := range msg.Payload {
			note(val)
		}
	}
	slices.Sort(named)
	return named
}

// eachObjectNamed visits the objects val, or each element of the collection it is, names: itself,
// the one a variant stands for, the one a structured value keeps, those an array holds.
func eachObjectNamed(val Value, visit func(int64)) {
	if val.Kind == ValSequence || val.Kind == ValSet {
		for _, elem := range listedElements(val) {
			eachObjectNamed(elem, visit)
		}
		return
	}
	if id := keptObject(val); id != 0 {
		visit(id)
	}
	switch val.Kind {
	case ValInstance, ValVariant:
		visit(val.Instance)
	case ValArray:
		for _, elem := range val.Array().Elements {
			eachObjectNamed(elem, visit)
		}
	}
}
