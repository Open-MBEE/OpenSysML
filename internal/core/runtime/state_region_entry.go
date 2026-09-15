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
	above     *lazyEntry        // the way down to container, entered by whichever branch gets there first
}

// lazyEntry is the chain of states a fork's branches still have to enter down to
// owner, the composite whose regions they enter.
type lazyEntry struct {
	chain []*ast.StateNode
	next  int
	owner *ast.StateNode
}

// enterRegionsInto activates one state per orthogonal region of container: the
// state branches names for that region, or the region's own start. container is
// nil for the machine's own regions.
func (e *StateExecutor) enterRegionsInto(container *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode) error {
	for _, region := range regions {
		entry := &regionEntry{region: region, container: container, branches: branches, target: branches[region]}
		if err := e.enterRegion(entry); err != nil {
			return err
		}
	}
	return nil
}

// enterForkBranches enters the owner's regions through the fork's branches: each
// runs its effect, enters the rest of the way down, then its target. Regions no
// branch enters start as usual; the owner's do behavior starts once all have.
func (e *StateExecutor) enterForkBranches(plan *lower.ForkPlan, toOwner []*ast.StateNode) error {
	var above *lazyEntry
	if len(toOwner) > 0 {
		above = &lazyEntry{chain: toOwner, owner: plan.Owner}
	}
	targets := plan.Targets()
	for _, region := range e.graph.CompositeStates[plan.Owner] {
		entry := &regionEntry{region: region, container: plan.Owner, branches: targets, above: above}
		if branch := plan.Branches[region]; branch != nil {
			entry.target = branch.Target.(*ast.StateNode)
			entry.branch = branch
		}
		if err := e.enterRegion(entry); err != nil {
			return err
		}
	}
	if above != nil {
		e.startDoAction(plan.Owner)
	}
	return nil
}

// enterRegion enters one region: its fork branch's effect, the states still on
// the way to its container, then the state it starts in.
func (e *StateExecutor) enterRegion(w *regionEntry) error {
	if w.branch != nil {
		for _, behavior := range w.branch.Effect {
			if err := e.executeBehavior(behavior); err != nil {
				return fmt.Errorf("fork branch effect: %w", err)
			}
		}
	}
	if w.above != nil {
		if err := e.enterLazily(w.above); err != nil {
			return err
		}
	}
	entry, err := e.regionStart(w)
	if err != nil {
		return err
	}
	e.activeConfig.regionStates[w.region] = entry
	// A target may lie below the region's own substates: every state on the way
	// down is entered, outermost first.
	for _, descendant := range e.descendantChain(w.container, entry) {
		if err := e.enterStateInto(descendant, w.branches); err != nil {
			return fmt.Errorf("enter starting state in region %s: %w", w.region.Name, err)
		}
	}
	// A composite entry starts where its own entry transitions choose, which is
	// then the deepest state the region keeps active.
	deepest, err := e.enterStartOf(entry)
	if err != nil {
		return err
	}
	if branch, ok := e.branchesTo(nil, deepest)[w.region]; ok {
		e.activeConfig.regionStates[w.region] = branch
	}
	return nil
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

// enterLazily enters what is left of the way down to the owner, which is
// activated without its regions and do behavior: the branches supply those.
func (e *StateExecutor) enterLazily(l *lazyEntry) error {
	for ; l.next < len(l.chain); l.next++ {
		state := l.chain[l.next]
		if state != l.owner {
			if err := e.enterState(state); err != nil {
				return fmt.Errorf("enter state %s: %w", state.Name, err)
			}
			continue
		}
		if err := e.activateState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
	}
	return nil
}
