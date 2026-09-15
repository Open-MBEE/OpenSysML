package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// Transition footprints are the static projection of what firing one transition
// may touch, beside the action footprints: the reaction reads its guard, its
// trigger's condition and the activity of its source; writes its effects' targets
// and the activity of every state it may exit or enter, with those states' exit
// and entry behaviors; sends; and queues the completion an entered state raises.
// A route through a choice or junction is followed along every branch, so the
// footprint covers whichever branch fires.

// TransitionFootprints is transition out of a state → what firing it may touch,
// its route through pseudostates followed along every branch. It is computed
// from the lowered graph on first use and shared by every caller.
func (g *StateGraph) TransitionFootprints() map[*Transition]Footprint {
	g.lowerFootprints()
	return g.transitionFootprints
}

// BehaviorFootprints is entry, do or exit behavior (its Node) → what running it
// touches, computed on first use as TransitionFootprints is.
func (g *StateGraph) BehaviorFootprints() map[ast.Node]Footprint {
	g.lowerFootprints()
	return g.behaviorFootprints
}

// lowerFootprints computes both footprint tables once; the behaviors' first,
// since a transition's footprint folds in the behaviors of the states it crosses.
func (g *StateGraph) lowerFootprints() {
	g.footprintsOnce.Do(func() {
		g.behaviorFootprints = make(map[ast.Node]Footprint)
		for _, behaviors := range g.Behaviors {
			if behaviors == nil {
				continue
			}
			for _, group := range [][]StateBehavior{behaviors.Entry, behaviors.Do, behaviors.Exit} {
				for _, behavior := range group {
					g.behaviorFootprints[behavior.Node] = behaviorFootprint(g, behavior)
				}
			}
		}
		g.transitionFootprints = make(map[*Transition]Footprint)
		for source, transitions := range g.Transitions {
			if _, isState := source.(*ast.StateNode); !isState {
				continue
			}
			for _, trans := range transitions {
				g.transitionFootprints[trans] = transitionFootprint(g, trans)
			}
		}
	})
}

// behaviorFootprint is what running the behavior's statements touches, and the
// activity of the state it belongs to, which stopping it writes.
func behaviorFootprint(graph *StateGraph, behavior StateBehavior) Footprint {
	b := &footprintBuilder{}
	b.statements(behavior.Body)
	if behavior.Owner != nil {
		b.read(graph.activity(behavior.Owner))
	}
	return b.footprint
}

// activity is the place standing for whether the state is active.
func (g *StateGraph) activity(state *ast.StateNode) Place {
	return Place{Name: state.Name, State: state}
}

// transitionFootprint projects the footprint of a transition out of a state.
func transitionFootprint(graph *StateGraph, trans *Transition) Footprint {
	b := &stateFootprintBuilder{footprintBuilder: &footprintBuilder{}, graph: graph, crossed: make(map[*ast.PseudostateNode]bool)}
	source := trans.Source.(*ast.StateNode)
	b.read(graph.activity(source))
	b.trigger(trans)
	b.segment(source, trans)
	return b.footprint
}

type stateFootprintBuilder struct {
	*footprintBuilder
	graph *StateGraph
	// crossed are the pseudostates followed, so a cycle through one ends.
	crossed map[*ast.PseudostateNode]bool
}

// trigger adds what the trigger reads: a change condition or a time duration. An
// accept or call trigger takes its event from the machine's own queue, where the
// send that queued it already touched the bus, so it is not an Accept here.
func (b *stateFootprintBuilder) trigger(trans *Transition) {
	switch t := trans.Trigger.(type) {
	case nil, *ast.AcceptEvent, *ast.CallEvent:
	case *ast.ChangeEvent:
		b.reads(trans.Scope, t.Condition)
	case *ast.TimeEvent:
		b.reads(trans.Scope, t.Duration)
	default:
		b.footprint.Dynamic = true
	}
}

// segment adds one segment of a compound transition, leaving source: its guard
// and effects, then where it ends — a state, or on through a pseudostate.
func (b *stateFootprintBuilder) segment(source *ast.StateNode, seg *Transition) {
	b.reads(seg.BodyScope, seg.Guard)
	for _, effect := range seg.Effect {
		b.statements(effect.Body)
	}
	switch target := seg.Target.(type) {
	case *ast.StateNode:
		b.move(source, target)
	case *ast.PseudostateNode:
		b.pseudostate(source, seg, target)
	default:
		b.footprint.Dynamic = true
	}
}

// pseudostate adds what a route through the pseudostate may do: every branch of
// a choice or junction, every target of a fork with the regions it leaves to
// start by default, the target of a join with the regions it leaves, the whole
// composite a history re-enters.
func (b *stateFootprintBuilder) pseudostate(source *ast.StateNode, seg *Transition, ps *ast.PseudostateNode) {
	if b.crossed[ps] {
		return
	}
	b.crossed[ps] = true
	switch ps.Kind {
	case ast.PseudostateChoice, ast.PseudostateJunction:
		for _, branch := range b.graph.Transitions[ps] {
			b.segment(source, branch)
		}
	case ast.PseudostateFork:
		for _, branch := range b.graph.Transitions[ps] {
			b.reads(branch.BodyScope, branch.Guard)
			for _, effect := range branch.Effect {
				b.statements(effect.Body)
			}
			if target, ok := branch.Target.(*ast.StateNode); ok {
				b.move(source, target)
			} else {
				b.footprint.Dynamic = true
			}
		}
		if plan := b.graph.ForkPlans[ps]; plan != nil {
			for _, region := range b.graph.CompositeStates[plan.Owner] {
				if plan.Branches[region] == nil {
					b.entersRegion(plan.Owner, region)
				}
			}
		}
	case ast.PseudostateJoin:
		if owner := b.graph.PseudostateOwner[ps]; owner != nil {
			b.exits(owner)
		}
		for _, branch := range b.graph.Transitions[ps] {
			b.segment(source, branch)
		}
	case ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
		owner := b.graph.PseudostateOwner[ps]
		if owner == nil {
			b.footprint.Dynamic = true
			return
		}
		b.move(source, owner)
		b.enters(owner)
	default:
		b.footprint.Dynamic = true
	}
}

// move adds the states a move from source to target may exit and enter: from the
// source up to below their common ancestor, and from there down to the target
// with whatever default entry reaches below it.
func (b *stateFootprintBuilder) move(source, target *ast.StateNode) {
	boundary := b.graph.commonAncestor(source, target)
	if b.graph.encloses(source, target) {
		boundary = b.graph.ParentState[source]
	}
	for state := source; state != nil && state != boundary; state = b.graph.ParentState[state] {
		b.exits(state)
	}
	for state := target; state != nil && state != boundary; state = b.graph.ParentState[state] {
		b.enters(state)
	}
	b.entersBelow(target)
}

// exits writes the activity of the state and of every state below it, and adds
// their exit behaviors.
func (b *stateFootprintBuilder) exits(state *ast.StateNode) {
	b.write(b.graph.activity(state))
	if behaviors := b.graph.Behaviors[state]; behaviors != nil {
		for _, behavior := range behaviors.Exit {
			b.merge(b.graph.behaviorFootprints[behavior.Node])
		}
	}
	for _, child := range b.graph.children(state) {
		b.exits(child)
	}
}

// enters writes the activity of the state, adds its entry behaviors, and the
// completion entering it may queue: its own completion transitions' guards
// read on entry, or the region it completes.
func (b *stateFootprintBuilder) enters(state *ast.StateNode) {
	b.write(b.graph.activity(state))
	if behaviors := b.graph.Behaviors[state]; behaviors != nil {
		for _, behavior := range behaviors.Entry {
			b.merge(b.graph.behaviorFootprints[behavior.Node])
		}
	}
	if b.graph.Completes(state) {
		b.footprint.Completion = true
	}
	for _, trans := range b.graph.Transitions[state] {
		if trans.Trigger == nil {
			b.footprint.Completion = true
			b.reads(trans.BodyScope, trans.Guard)
		}
	}
}

// entersBelow adds every state below the target that entering it may enter: a
// composite's default entry reaches its initial states, a history any of them.
func (b *stateFootprintBuilder) entersBelow(state *ast.StateNode) {
	for _, child := range b.graph.children(state) {
		b.enters(child)
		b.entersBelow(child)
	}
}

// entersRegion adds every state a region of owner may start in by default: the
// graph-only state standing for the region, or the states it declares, and all
// below them.
func (b *stateFootprintBuilder) entersRegion(owner *ast.StateNode, region *ast.StateRegion) {
	if wrapper := b.graph.RegionState[region]; wrapper != nil {
		b.enters(wrapper)
		b.entersBelow(wrapper)
		return
	}
	for _, child := range b.graph.children(owner) {
		if b.graph.RegionOf[child] == region {
			b.enters(child)
			b.entersBelow(child)
		}
	}
}

// children lists the states whose parent is state, in graph order.
func (g *StateGraph) children(state *ast.StateNode) []*ast.StateNode {
	var children []*ast.StateNode
	for _, candidate := range g.States {
		if g.ParentState[candidate] == state {
			children = append(children, candidate)
		}
	}
	for candidate, parent := range g.ParentState {
		if parent == state && g.HiddenStates[candidate] {
			children = append(children, candidate)
		}
	}
	return children
}

// commonAncestor is the innermost state enclosing both, nil when only the
// machine does.
func (g *StateGraph) commonAncestor(a, b *ast.StateNode) *ast.StateNode {
	above := make(map[*ast.StateNode]bool)
	for state := g.ParentState[a]; state != nil; state = g.ParentState[state] {
		above[state] = true
	}
	for state := g.ParentState[b]; state != nil; state = g.ParentState[state] {
		if above[state] {
			return state
		}
	}
	return nil
}

// encloses reports whether target is source or lies below it.
func (g *StateGraph) encloses(source, target *ast.StateNode) bool {
	for state := target; state != nil; state = g.ParentState[state] {
		if state == source {
			return true
		}
	}
	return false
}
