package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// regionEntry is one orthogonal region's share of entering its container.
type regionEntry struct {
	region    *ast.StateRegion
	container *ast.StateNode
	branches  map[*ast.StateRegion]*ast.StateNode
	target    *ast.StateNode    // where the region starts instead of its own start, if anywhere
	branch    *lower.Transition // the fork branch into the region, whose effect runs first, if any
}

// lazyEntry is the chain of states a fork's branches still have to enter down to
// owner, the composite whose regions they enter.
type lazyEntry struct {
	chain    []*ast.StateNode
	next     int
	owner    *ast.StateNode
	branches map[*ast.StateRegion]*ast.StateNode // region the chain passes through → the state it passes into
}

// forkEntry describes the way from boundary, which stays active, down to the
// composite whose regions a fork's branches enter.
func (e *StateExecutor) forkEntry(boundary, owner *ast.StateNode) *lazyEntry {
	return &lazyEntry{
		chain:    e.descendantChain(boundary, owner),
		owner:    owner,
		branches: e.branchesTo(boundary, owner),
	}
}

// enterRegionsInto activates one state per orthogonal region of container: the
// state branches names for that region, or the region's own start. container is
// nil for the machine's own regions.
func (e *StateExecutor) enterRegionsInto(container *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode) error {
	entries := make([]*regionEntry, 0, len(regions))
	for _, region := range regions {
		entries = append(entries, &regionEntry{region: region, container: container, branches: branches, target: branches[region]})
	}
	return e.enterRegions(container, entries, true)
}

// enterRegions enters the regions as queues of an entry front, each queue the
// region's units in their order: queues of the front under way when a queue of
// it is running, which waits for them when wait says, or a front of their own,
// drawn one unit at a time; one region alone is entered as it stands.
func (e *StateExecutor) enterRegions(container *ast.StateNode, entries []*regionEntry, wait bool) error {
	bodies := make([]func() error, len(entries))
	for i, entry := range entries {
		bodies[i] = func() error { return e.enterRegion(entry) }
	}
	where := enteringWherePrefix + e.stateMachine.Name
	if container != nil {
		where = enteringWherePrefix + container.Name
	}
	return e.performUnits(ChoiceEntryOrder, where, bodies, wait)
}

// enterForkBranches enters the owner's regions through the fork's branches, one
// queue per region: its branch's effect, the rest of the way down to the owner —
// entered once, by whichever branch is drawn first — then its target. A region
// no branch enters starts as usual once the way down is entered. The do
// behaviors of the states entered on the way down start once all have,
// innermost first, as after an ordinary entry.
func (e *StateExecutor) enterForkBranches(fork *ast.PseudostateNode, plan *lower.ForkPlan, above *lazyEntry) error {
	targets := plan.Targets()
	regions := e.graph.CompositeStates[plan.Owner]
	bodies := make([]func() error, 0, len(regions))
	for _, region := range regions {
		entry := &regionEntry{region: region, container: plan.Owner, branches: targets}
		branch := plan.Branches[region]
		if branch != nil {
			entry.target = branch.Target.(*ast.StateNode)
		}
		bodies = append(bodies, func() error {
			if branch == nil {
				if err := e.await(ChoiceEntryOrder, func() bool { return above.next >= len(above.chain) }); err != nil {
					return err
				}
			} else if err := e.runBranchEffect(branch); err != nil {
				return err
			}
			if err := e.enterShared(above); err != nil {
				return err
			}
			return e.enterRegion(entry)
		})
	}
	if err := e.drawUnits(ChoiceEntryOrder, forkWherePrefix+fork.Name, bodies); err != nil {
		return err
	}
	for i := len(above.chain) - 1; i >= 0; i-- {
		e.startDoAction(above.chain[i])
	}
	return nil
}

// enterShared enters the rest of the way down the fork's branches share, each
// state a unit every branch heads until one of them enters it.
func (e *StateExecutor) enterShared(l *lazyEntry) error {
	for i := l.next; i < len(l.chain); i++ {
		state := l.chain[i]
		perform := true
		if !e.graph.HiddenStates[state] {
			head := unitHead{label: entryLabel(state), at: state, shared: state, dropped: func() bool { return l.next > i }}
			var err error
			if perform, err = e.unit(ChoiceEntryOrder, head); err != nil {
				return err
			}
		}
		if perform && l.next <= i {
			if err := e.enterLazily(l, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// runBranchEffect executes a fork branch's effect, if any, as one unit.
func (e *StateExecutor) runBranchEffect(branch *lower.Transition) error {
	if branch == nil {
		return nil
	}
	if _, err := e.unit(ChoiceEntryOrder, unitHead{label: effectLabel(branch), at: branch.Decl}); err != nil {
		return err
	}
	for _, behavior := range branch.Effect {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("fork branch effect: %w", err)
		}
	}
	return nil
}

// enterRegion enters one region: the state it starts in, entering every state
// on the way down to it as a transition does. A start its entry guards decide
// is drawn before they are read, so they read what the units before it wrote.
func (e *StateExecutor) enterRegion(w *regionEntry) error {
	if w.target == nil && e.graph.RegionState[w.region] == nil && len(e.graph.StartOf(w.region)) > 0 {
		if err := e.unitAhead(ChoiceEntryOrder, e.startHead(w.region, w.container)); err != nil {
			return err
		}
	}
	entry, err := e.regionStart(w)
	if err != nil {
		return err
	}
	e.activeConfig.regionStates[w.region] = entry
	_, deepest, err := e.enterToward(w.container, entry, w.branches)
	if err != nil {
		return fmt.Errorf("enter starting state in region %s: %w", w.region.Name, err)
	}
	// The deepest state the region keeps active is the one on the way down that
	// its own substates declare.
	if branch, ok := e.branchesTo(nil, deepest)[w.region]; ok {
		e.activeConfig.regionStates[w.region] = branch
	}
	return nil
}

// startHead names the unit that enters where body starts: the entry of the first
// state on the way below above when the first start transition has no guard, else
// the start itself, its state known once its guards are read.
func (e *StateExecutor) startHead(body ast.Node, above *ast.StateNode) unitHead {
	starts := e.graph.StartOf(body)
	if len(starts) > 0 && starts[0].Guard == nil {
		for _, state := range e.descendantChain(above, starts[0].Target) {
			if !e.graph.HiddenStates[state] {
				return unitHead{label: entryLabel(state), at: state}
			}
		}
	}
	return unitHead{label: "start of " + e.describeBody(body), at: body}
}

// regionStart is the state a region starts in: the target it was given, the
// substate of a parallel state it stands for, or its own entry transitions' choice.
func (e *StateExecutor) regionStart(w *regionEntry) (*ast.StateNode, error) {
	if w.target != nil {
		return w.target, nil
	}
	if owner := e.graph.RegionState[w.region]; owner != nil {
		return owner, nil
	}
	entry, err := e.startIn(w.region)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("region %s has no initial state", w.region.Name)
	}
	return entry, nil
}

// enterLazily enters the way down as far as the first upto states of the chain.
// A state on the way is activated without its do behavior; the region the chain
// passes through is left to the states below, its other regions start as usual
// — alongside the way down, when a front draws it — and the owner's regions are
// the branches' to enter.
func (e *StateExecutor) enterLazily(l *lazyEntry, upto int) error {
	for ; l.next < upto; l.next++ {
		state := l.chain[l.next]
		if err := e.activateState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
		if state == l.owner {
			continue
		}
		var others []*regionEntry
		for _, region := range e.graph.CompositeStates[state] {
			if _, onWay := l.branches[region]; onWay {
				continue
			}
			others = append(others, &regionEntry{region: region, container: state, branches: l.branches})
		}
		if err := e.enterRegions(state, others, false); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
	}
	return nil
}
