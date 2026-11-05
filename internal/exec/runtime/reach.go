package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// PathStep is one step of a root-relative object path: a feature of the object
// before it and, within a multi-valued feature, the position of the value taken,
// counted from 1 as an object path spells it (`wheels[2]`); 0 for a
// single-valued feature.
type PathStep struct {
	// Name is the feature as the holder's feature values key it, which an
	// object path addresses it by; Feature is that feature as declared.
	Name    string
	Feature *EffectiveFeature
	// Library marks a feature the standard library derives (`subparts`,
	// `ownedPorts`), which holds the object a declared feature also holds.
	Library bool
	// Index is the position within a multi-valued feature, from 1; 0 for a
	// single-valued one.
	Index  int
	Object *Instance
}

// Reach is how a traversal of an object graph reached one object: the object
// holding it, the feature of that holder it is a value of, and the steps from
// the root. The root's Reach holds no Holder and no Steps.
type Reach struct {
	Object *Instance
	Holder *Instance
	// Feature is the holder's feature the object is a value of, and Index the
	// position among that feature's values, as PathStep counts them.
	Feature *EffectiveFeature
	Index   int
	// Steps are the path from the root, the last of which reaches Object.
	Steps []PathStep
}

// Depth is how many steps the reach is from the root.
func (r Reach) Depth() int { return len(r.Steps) }

// Path spells the steps as an object path: the feature names joined by dots,
// each indexed within a multi-valued feature (`powertrain.engine.fuelIn`,
// `wheels[2].hub`); empty for the root.
func (r Reach) Path() string { return PathText(r.Steps) }

// PathText spells steps as an object path.
func PathText(steps []PathStep) string {
	var b strings.Builder
	for i, step := range steps {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(step.Name)
		if step.Index > 0 {
			fmt.Fprintf(&b, "[%d]", step.Index)
		}
	}
	return b.String()
}

// TraversalBounds is how far a traversal of an object graph goes: objects
// Depth steps from the root at most, and Objects objects in all. A zero bound
// is no bound.
type TraversalBounds struct {
	Depth   int
	Objects int
}

func (b TraversalBounds) within(depth, objects int) bool {
	return (b.Depth <= 0 || depth <= b.Depth) && (b.Objects <= 0 || objects < b.Objects)
}

// Reached is what Traverse reached.
type Reached struct {
	// Reaches are the objects reached, the root first, each once in the order
	// it was first reached.
	Reaches []Reach
	// Truncated reports that the object bound cut the traversal short.
	Truncated bool
}

// Traverse walks the objects reachable from root through their feature values
// and reports how each was reached. The order is deterministic: depth-first
// from the root, over the features of an object in name order and the values
// of a feature in their order, descending into an object the first time it is
// reached and never into one whose type an object already on the path from the
// root has (which bounds a recursive type). An object below bounds.Depth is
// reached but not descended into. A feature whose value cannot be read is
// skipped; the caller reading the values sees the error.
func (ctx *Context) Traverse(root *Instance, bounds TraversalBounds) Reached {
	var t Reached
	seen := make(map[int64]bool)
	onPath := make(map[*Instance]bool)
	typesOnPath := make(map[*symbols.Symbol]bool)

	var walk func(reach Reach)
	walk = func(reach Reach) {
		cur := reach.Object
		if seen[cur.ID] {
			return
		}
		if !bounds.within(reach.Depth(), len(t.Reaches)) {
			if bounds.Objects > 0 && len(t.Reaches) >= bounds.Objects {
				t.Truncated = true
			}
			return
		}
		seen[cur.ID] = true
		t.Reaches = append(t.Reaches, reach)
		if bounds.Depth > 0 && reach.Depth() >= bounds.Depth {
			return
		}
		onPath[cur] = true
		typesOnPath[cur.Type] = true
		defer func() {
			delete(onPath, cur)
			delete(typesOnPath, cur.Type)
		}()
		for _, step := range ctx.heldSteps(cur) {
			if typesOnPath[step.Object.Type] {
				continue
			}
			walk(Reach{
				Object:  step.Object,
				Holder:  cur,
				Feature: step.Feature,
				Index:   step.Index,
				Steps:   append(slices.Clip(reach.Steps), step),
			})
		}
	}
	walk(Reach{Object: root})
	return t
}

// PathBounds is how far PathsTo searches: paths of Depth steps at most, and
// Paths paths in all. A zero bound is no bound.
type PathBounds struct {
	Depth int
	Paths int
}

// Paths is what PathsTo found.
type Paths struct {
	// Steps are the paths, each the steps from the root to the target, in the
	// order Traverse would reach them; empty steps are the root itself.
	Steps [][]PathStep
	// Truncated reports that a bound cut the search short: more paths reach the
	// target than bounds.Paths, or a path cut at bounds.Depth ended at an object
	// from which the target is reachable, so a deeper path may.
	Truncated bool
}

// PathsTo lists every root-relative path that reaches target from root: every
// sequence of feature values that ends at target, none visiting an object
// twice nor, as Traverse, an object whose type an object already on the path
// has — aliases through connector ends and reference usages included, since
// those hold the object they refer to as a value. The paths come in the order
// Traverse walks: depth-first, features in name order, values in their order.
// The root itself is reached by the empty path and nothing is read for it.
func (ctx *Context) PathsTo(root, target *Instance, bounds PathBounds) Paths {
	var p Paths
	if root.ID == target.ID {
		p.Steps = [][]PathStep{nil}
		return p
	}
	onPath := make(map[int64]bool)
	typesOnPath := make(map[*symbols.Symbol]bool)
	var reachesTarget map[int64]bool
	if bounds.Depth > 0 {
		reachesTarget = ctx.holdersReaching(root, target)
	}

	var search func(cur *Instance, steps []PathStep)
	search = func(cur *Instance, steps []PathStep) {
		if cur.ID == target.ID {
			if bounds.Paths > 0 && len(p.Steps) >= bounds.Paths {
				p.Truncated = true
				return
			}
			p.Steps = append(p.Steps, slices.Clone(steps))
			return
		}
		if bounds.Depth > 0 && len(steps) >= bounds.Depth {
			if reachesTarget[cur.ID] {
				p.Truncated = true
			}
			return
		}
		onPath[cur.ID] = true
		typesOnPath[cur.Type] = true
		defer func() {
			delete(onPath, cur.ID)
			delete(typesOnPath, cur.Type)
		}()
		for _, step := range ctx.heldSteps(cur) {
			if step.Library || onPath[step.Object.ID] || typesOnPath[step.Object.Type] {
				continue
			}
			search(step.Object, append(slices.Clip(steps), step))
		}
	}
	search(root, nil)
	return p
}

// holdersReaching is the set of objects below root from which held values may
// lead to target, the graph walked once as Traverse walks it; an object the
// walk never looked into counts as reaching, so the set overstates, never
// understates, what a depth cutoff hides.
func (ctx *Context) holdersReaching(root, target *Instance) map[int64]bool {
	holders := make(map[int64][]int64)
	explored := map[int64]bool{root.ID: true}
	unexplored := make(map[int64]bool)
	typesOnPath := make(map[*symbols.Symbol]bool)
	var walk func(cur *Instance)
	walk = func(cur *Instance) {
		typesOnPath[cur.Type] = true
		defer delete(typesOnPath, cur.Type)
		for _, step := range ctx.heldSteps(cur) {
			if step.Library {
				continue
			}
			obj := step.Object
			holders[obj.ID] = append(holders[obj.ID], cur.ID)
			if explored[obj.ID] {
				continue
			}
			if typesOnPath[obj.Type] {
				unexplored[obj.ID] = true
				continue
			}
			explored[obj.ID] = true
			delete(unexplored, obj.ID)
			walk(obj)
		}
	}
	walk(root)
	reaches := make(map[int64]bool)
	pending := []int64{target.ID}
	for id := range unexplored {
		reaches[id] = true
		pending = append(pending, id)
	}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, holder := range holders[id] {
			if !reaches[holder] {
				reaches[holder] = true
				pending = append(pending, holder)
			}
		}
	}
	return reaches
}

// heldSteps are the objects inst holds as feature values, each as the step
// reaching it: the features the model declares in name order, then those the
// library derives (`subparts`, `ownedPorts`) in name order, the values of a
// multi-valued feature in their order. A feature is stepped through under its
// own name only, not the name of a feature it redefines, which keys the same
// value.
func (ctx *Context) heldSteps(inst *Instance) []PathStep {
	var declared, library []PathStep
	idx := ctx.model.resolver.Index()
	for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil || fv.Feature == nil || fv.Feature.Name != name {
			continue
		}
		step := PathStep{Name: name, Feature: fv.Feature}
		if fv.Feature.Symbol != nil && idx.IsLibraryDocument(fv.Feature.Symbol.DocName) {
			step.Library = true
		}
		var steps []PathStep
		if fv.Feature.Scalar() {
			if !fv.Materialized {
				continue
			}
			for _, obj := range ctx.heldObjects(fv.Value) {
				step.Object = obj
				steps = append(steps, step)
			}
		} else {
			elements, err := ctx.valueElements(fv.Values)
			if err != nil {
				continue
			}
			for i, elem := range elements {
				for _, obj := range ctx.heldObjects(elem) {
					step.Index, step.Object = i+1, obj
					steps = append(steps, step)
				}
			}
		}
		if step.Library {
			library = append(library, steps...)
		} else {
			declared = append(declared, steps...)
		}
	}
	return append(declared, library...)
}

// valueElements are the elements of a collection value, a sequence's as held.
func (ctx *Context) valueElements(val Value) ([]Value, error) {
	switch val.Kind {
	case ValSequence:
		if val.Sequence() != nil {
			return ctx.HeldElements(val)
		}
	case ValSet:
		if val.Set() != nil {
			return val.Set().Elements(), nil
		}
	case ValArray:
		if val.Array() != nil {
			return val.Array().Elements, nil
		}
	}
	return nil, nil
}

// heldObjects are the objects a value denotes: the instance it is, the object
// a selected variant materialized, or those of its elements.
func (ctx *Context) heldObjects(val Value) []*Instance {
	// The empty object a valueless value-typed feature materializes is no
	// value: every surface reads it as unset, so no path reaches it.
	if ctx.HoldsNoValue(val) {
		return nil
	}
	if id, ok := val.Object(); ok {
		if inst, found := ctx.Instance(id); found {
			return []*Instance{inst}
		}
		return nil
	}
	elements, err := ctx.valueElements(val)
	if err != nil {
		return nil
	}
	var objects []*Instance
	for _, elem := range elements {
		objects = append(objects, ctx.heldObjects(elem)...)
	}
	return objects
}
