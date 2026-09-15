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
	for _, region := range regions {
		entry := &regionEntry{region: region, container: container, branches: branches, target: branches[region]}
		if err := e.enterRegion(entry); err != nil {
			return err
		}
	}
	return nil
}

// enterForkBranches enters the owner's regions through the fork's branches: the
// first branch, in region order, runs its effect and enters the rest of the way
// down, then every region enters in declaration order, each branch's effect
// before its target, a region no branch enters starting as usual. The do
// behaviors of the states entered on the way down start once all have,
// innermost first, as after an ordinary entry.
func (e *StateExecutor) enterForkBranches(plan *lower.ForkPlan, above *lazyEntry) error {
	targets := plan.Targets()
	regions := e.graph.CompositeStates[plan.Owner]
	var first *lower.Transition
	for _, region := range regions {
		if first = plan.Branches[region]; first != nil {
			break
		}
	}
	if err := e.runBranchEffect(first); err != nil {
		return err
	}
	if err := e.enterLazily(above, len(above.chain)); err != nil {
		return err
	}
	for _, region := range regions {
		entry := &regionEntry{region: region, container: plan.Owner, branches: targets}
		if branch := plan.Branches[region]; branch != nil {
			entry.target = branch.Target.(*ast.StateNode)
			if branch != first {
				entry.branch = branch
			}
		}
		if err := e.enterRegion(entry); err != nil {
			return err
		}
	}
	for i := len(above.chain) - 1; i >= 0; i-- {
		e.startDoAction(above.chain[i])
	}
	return nil
}

// runBranchEffect executes a fork branch's effect, if any.
func (e *StateExecutor) runBranchEffect(branch *lower.Transition) error {
	if branch == nil {
		return nil
	}
	for _, behavior := range branch.Effect {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("fork branch effect: %w", err)
		}
	}
	return nil
}

// enterRegion enters one region: its fork branch's effect, then the state it
// starts in, entering every state on the way down to it as a transition does.
func (e *StateExecutor) enterRegion(w *regionEntry) error {
	if err := e.runBranchEffect(w.branch); err != nil {
		return err
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
// passes through is left to the states below, its other regions start as usual,
// and the owner's regions are the branches' to enter.
func (e *StateExecutor) enterLazily(l *lazyEntry, upto int) error {
	for ; l.next < upto; l.next++ {
		state := l.chain[l.next]
		if err := e.activateState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
		if state == l.owner {
			continue
		}
		for _, region := range e.graph.CompositeStates[state] {
			if _, onWay := l.branches[region]; onWay {
				continue
			}
			entry := &regionEntry{region: region, container: state, branches: l.branches}
			if err := e.enterRegion(entry); err != nil {
				return fmt.Errorf("enter state %s: %w", state.Name, err)
			}
		}
	}
	return nil
}
