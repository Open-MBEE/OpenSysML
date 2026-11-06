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
	var known *reachability
	if bounds.Depth > 0 {
		known = ctx.holdersReaching(root, target)
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
			if known.hides(cur, onPath, typesOnPath) {
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

// reachability is what a walk of a root's graph, as Traverse walks it, knows
// about which objects lead to a target; it walks further only when asked.
type reachability struct {
	ctx        *Context
	target     *Instance
	holders    map[int64][]int64     // the objects holding each object the walk saw
	held       map[int64][]*Instance // what each object the walk looked into holds
	explored   map[int64]bool        // looked into
	unexplored map[int64]bool        // held, but kept out of by the type guard
	reaches    map[int64]bool        // leads to the target through objects looked into
	below      map[int64][]*Instance // the unexplored objects below an object, once asked
}

// holdersReaching walks root's graph once, as Traverse does, and records which
// objects lead to target and which it was kept out of.
func (ctx *Context) holdersReaching(root, target *Instance) *reachability {
	r := &reachability{
		ctx:        ctx,
		target:     target,
		holders:    make(map[int64][]int64),
		held:       make(map[int64][]*Instance),
		explored:   map[int64]bool{root.ID: true},
		unexplored: make(map[int64]bool),
		below:      make(map[int64][]*Instance),
	}
	r.walk(root, make(map[*symbols.Symbol]bool))
	r.propagate()
	return r
}

// walk looks into cur and, under the type guard, into what it holds.
func (r *reachability) walk(cur *Instance, typesOnPath map[*symbols.Symbol]bool) {
	typesOnPath[cur.Type] = true
	defer delete(typesOnPath, cur.Type)
	for _, step := range r.ctx.heldSteps(cur) {
		if step.Library {
			continue
		}
		obj := step.Object
		r.held[cur.ID] = append(r.held[cur.ID], obj)
		r.holders[obj.ID] = append(r.holders[obj.ID], cur.ID)
		if r.explored[obj.ID] {
			continue
		}
		if typesOnPath[obj.Type] {
			r.unexplored[obj.ID] = true
			continue
		}
		r.explored[obj.ID] = true
		delete(r.unexplored, obj.ID)
		r.walk(obj, typesOnPath)
	}
}

// propagate recomputes reaches from the holders seen so far.
func (r *reachability) propagate() {
	r.reaches = map[int64]bool{r.target.ID: true}
	pending := []int64{r.target.ID}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, holder := range r.holders[id] {
			if !r.reaches[holder] {
				r.reaches[holder] = true
				pending = append(pending, holder)
			}
		}
	}
}

// hides reports whether a path cut at cur may go on to the target, looking
// into the unexplored objects the path could step to below cur until it knows.
func (r *reachability) hides(cur *Instance, onPath map[int64]bool, typesOnPath map[*symbols.Symbol]bool) bool {
	for {
		if r.reaches[cur.ID] {
			return true
		}
		var enter *Instance
		var route map[*symbols.Symbol]bool
		if r.unexplored[cur.ID] {
			enter = cur
		} else {
			enter, route = r.enterable(cur, onPath, typesOnPath)
		}
		if enter == nil {
			return false
		}
		types := maps.Clone(typesOnPath)
		types[cur.Type] = true
		maps.Copy(types, route)
		r.explored[enter.ID] = true
		delete(r.unexplored, enter.ID)
		r.walk(enter, types)
		r.propagate()
		clear(r.below)
	}
}

// enterable is an unexplored object below cur the cut path could step to, by
// objects not on the path and of no type on it, cur's or its own, and that route's types.
func (r *reachability) enterable(cur *Instance, onPath map[int64]bool, typesOnPath map[*symbols.Symbol]bool) (*Instance, map[*symbols.Symbol]bool) {
	tried := make(map[*symbols.Symbol]bool)
	for _, want := range r.unexploredBelow(cur) {
		if tried[want.Type] || typesOnPath[want.Type] || want.Type == cur.Type {
			continue
		}
		tried[want.Type] = true
		parent := map[int64]*Instance{cur.ID: nil}
		stack := []*Instance{cur}
		for len(stack) > 0 {
			next := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, obj := range r.held[next.ID] {
				if _, seen := parent[obj.ID]; seen || onPath[obj.ID] || typesOnPath[obj.Type] || obj.Type == cur.Type {
					continue
				}
				if r.unexplored[obj.ID] {
					if obj.Type != want.Type {
						continue
					}
					route := make(map[*symbols.Symbol]bool)
					for x := next; x != cur; x = parent[x.ID] {
						route[x.Type] = true
					}
					return obj, route
				}
				if obj.Type == want.Type {
					continue
				}
				parent[obj.ID] = next
				stack = append(stack, obj)
			}
		}
	}
	return nil, nil
}

// unexploredBelow lists the unexplored objects held somewhere below cur.
func (r *reachability) unexploredBelow(cur *Instance) []*Instance {
	if found, asked := r.below[cur.ID]; asked {
		return found
	}
	var found []*Instance
	seen := map[int64]bool{cur.ID: true}
	stack := []*Instance{cur}
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, obj := range r.held[next.ID] {
			if seen[obj.ID] {
				continue
			}
			seen[obj.ID] = true
			if r.unexplored[obj.ID] {
				found = append(found, obj)
			} else {
				stack = append(stack, obj)
			}
		}
	}
	r.below[cur.ID] = found
	return found
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
