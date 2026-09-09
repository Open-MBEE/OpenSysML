package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// transientPseudostate reports whether a pseudostate merely routes a transition
// onwards — choice, junction, entry point and exit point — as opposed to fork,
// join and history, which rewrite the whole active configuration.
func transientPseudostate(kind ast.PseudostateKind) bool {
	switch kind {
	case ast.PseudostateChoice, ast.PseudostateJunction, ast.PseudostateEntry, ast.PseudostateExit:
		return true
	}
	return false
}

// transitionTarget returns the state a transition ends at, following any chain of
// transient pseudostates on the way.
func (e *StateExecutor) transitionTarget(trans *lower.Transition) (*ast.StateNode, error) {
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		return target, nil
	case *ast.PseudostateNode:
		return e.pseudostateTarget(target)
	default:
		return nil, fmt.Errorf("transition target must be a state or pseudostate, got %T", trans.Target)
	}
}

// pseudostateTarget follows the outgoing transitions of a transient pseudostate
// until a state is reached, so a chain such as exit point → junction → state ends
// at the state the compound transition actually enters.
//
// A choice's guards are evaluated when it is reached and a junction's when its
// incoming transition is evaluated; both are evaluated here, in declaration
// order, which makes the two indistinguishable for a guard over state data.
func (e *StateExecutor) pseudostateTarget(ps *ast.PseudostateNode) (*ast.StateNode, error) {
	visited := make(map[*ast.PseudostateNode]bool)
	for {
		if visited[ps] {
			return nil, fmt.Errorf("%s %s: outgoing transitions form a cycle between pseudostates", ps.Kind, ps.Name)
		}
		visited[ps] = true

		branch, err := e.pseudostateBranch(ps)
		if err != nil {
			return nil, err
		}
		switch target := branch.Target.(type) {
		case *ast.StateNode:
			return target, nil
		case *ast.PseudostateNode:
			if !transientPseudostate(target.Kind) {
				return nil, fmt.Errorf("%s %s: a transition into %s %s is not supported", ps.Kind, ps.Name, target.Kind, target.Name)
			}
			ps = target
		default:
			return nil, fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", ps.Kind, ps.Name, branch.Target)
		}
	}
}

// pseudostateBranch returns the outgoing transition a pseudostate routes along:
// the first whose guard is satisfied, in declaration order, an unguarded one
// being the default branch. Exactly one succession is taken, as KerML
// `DecisionPerformance::outgoingHBLink: HappensBefore[1]` requires.
func (e *StateExecutor) pseudostateBranch(ps *ast.PseudostateNode) (*lower.Transition, error) {
	outgoing := e.graph.Transitions[ps]
	if len(outgoing) == 0 {
		return nil, fmt.Errorf("%s %s has no outgoing transitions", ps.Kind, ps.Name)
	}
	for _, trans := range outgoing {
		pass, err := e.passesGuard(trans)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", ps.Kind, ps.Name, err)
		}
		if pass {
			return trans, nil
		}
	}
	return nil, fmt.Errorf("%s %s: no guard evaluated to true", ps.Kind, ps.Name)
}

// regionContains reports whether state is declared in region or nested below a
// state that is.
func (e *StateExecutor) regionContains(region *ast.StateRegion, state *ast.StateNode) bool {
	if region == nil || state == nil {
		return false
	}
	for _, ancestor := range e.getParentChain(state) {
		if e.graph.RegionOf[ancestor] == region {
			return true
		}
	}
	return false
}

// declaringRegion returns the region a state belongs to, which for a graph-only
// region owner is the region it stands for.
func (e *StateExecutor) declaringRegion(state *ast.StateNode) *ast.StateRegion {
	if region := e.graph.RegionOf[state]; region != nil {
		return region
	}
	return e.graph.HiddenRegionOf[state]
}

// branchesTo names, for every orthogonal region on the path from `from` down to
// target, the deepest state of that path inside it. Entering `from` with those
// branches therefore ends at target instead of at the regions' initial states.
func (e *StateExecutor) branchesTo(from, target *ast.StateNode) map[*ast.StateRegion]*ast.StateNode {
	branches := make(map[*ast.StateRegion]*ast.StateNode)
	var region *ast.StateRegion
	for _, state := range e.descendantChain(from, target) {
		if declaring := e.declaringRegion(state); declaring != nil {
			region = declaring
		}
		if region != nil {
			branches[region] = state
		}
	}
	return branches
}

// entryPlan returns the state to enter below lca in order to reach target,
// together with the region branches that get there. Entering stops at the
// outermost state on the path whose own orthogonal regions the path descends
// into, because entering that state enters the rest of the path through them.
func (e *StateExecutor) entryPlan(lca, target *ast.StateNode) (*ast.StateNode, map[*ast.StateRegion]*ast.StateNode) {
	chain := e.descendantChain(lca, target)
	for i := 0; i+1 < len(chain); i++ {
		region := e.declaringRegion(chain[i+1])
		if region != nil && e.graph.RegionOwner[region] == chain[i] {
			return chain[i], e.branchesTo(chain[i], target)
		}
	}
	return target, nil
}

// activeLeavesBelow returns the deepest active states inside state — one per
// active orthogonal region, recursively — which are the states whose outgoing
// transitions have to be scheduled after entering it.
func (e *StateExecutor) activeLeavesBelow(state *ast.StateNode) []*ast.StateNode {
	regions, composite := e.graph.CompositeStates[state]
	if !composite {
		return []*ast.StateNode{state}
	}
	leaves := make([]*ast.StateNode, 0, len(regions))
	for _, region := range regions {
		active, ok := e.activeConfig.regionStates[region]
		if !ok || active == state {
			continue
		}
		leaves = append(leaves, e.activeLeavesBelow(active)...)
	}
	return leaves
}

// scheduleFromEntered schedules the outgoing transitions of every state the move
// just made active below and including state.
func (e *StateExecutor) scheduleFromEntered(state *ast.StateNode) error {
	for _, leaf := range e.activeLeavesBelow(state) {
		if err := e.scheduleFromLeaf(leaf); err != nil {
			return fmt.Errorf("schedule transitions: %w", err)
		}
	}
	return nil
}

// runEffect performs the actions of a transition's effect, in order.
func (e *StateExecutor) runEffect(trans *lower.Transition) error {
	for _, behavior := range trans.Effect {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	return nil
}

// fireTransitionInRegion fires a transition whose source is the active state of
// an orthogonal region. A target inside the same region moves only that region;
// a target outside it leaves the whole region set, which is what makes a
// transition through a choice, junction or entry/exit point reachable from
// inside a region.
func (e *StateExecutor) fireTransitionInRegion(region *ast.StateRegion, trans *lower.Transition) (bool, error) {
	// Fork, join and history replace the entire active configuration rather than
	// move one region, so they are fired whole.
	if isSynchronizationTarget(trans.Target) {
		return e.fireTransition(trans)
	}

	pass, err := e.passesGuard(trans)
	if err != nil || !pass {
		return false, err
	}
	e.transitionDecided()

	target, err := e.transitionTarget(trans)
	if err != nil {
		return false, err
	}
	if target == nil {
		return false, fmt.Errorf("transition out of region %s has no target state", region.Name)
	}

	source := e.activeConfig.regionStates[region]
	if e.regionContains(region, target) {
		return true, e.moveBetweenRegions(region, region, source, trans, target)
	}
	if exit, sibling := e.concurrentRegionsFor(region, target); sibling != nil {
		return true, e.moveBetweenRegions(exit, e.innermostActiveRegion(sibling, target), source, trans, target)
	}
	return true, e.leaveRegion(region, trans, target)
}

// concurrentRegionsFor finds the level at which target lies in a region concurrent
// with region: it returns the region on the source side at that level — the one to
// leave, which is region itself or an enclosing one — and the region holding target.
// Both are nil when target lies outside every enclosing region set.
func (e *StateExecutor) concurrentRegionsFor(region *ast.StateRegion, target *ast.StateNode) (*ast.StateRegion, *ast.StateRegion) {
	for current := region; current != nil; {
		if sibling := e.siblingRegionContaining(current, target); sibling != nil {
			return current, sibling
		}
		owner := e.graph.RegionOwner[current]
		if owner == nil {
			return nil, nil
		}
		current = e.enclosingRegion(owner)
	}
	return nil, nil
}

// enclosingRegion returns the region state is declared in, or the one declaring
// its nearest ancestor when state is a substate, and nil when neither is in one.
func (e *StateExecutor) enclosingRegion(state *ast.StateNode) *ast.StateRegion {
	for current := state; current != nil; current = e.graph.ParentState[current] {
		if region := e.graph.RegionOf[current]; region != nil {
			return region
		}
	}
	return nil
}

// isBelowOrEqual reports whether state is ancestor itself or nested below it.
func (e *StateExecutor) isBelowOrEqual(state, ancestor *ast.StateNode) bool {
	for current := state; current != nil; current = e.graph.ParentState[current] {
		if current == ancestor {
			return true
		}
	}
	return false
}

// siblingRegionContaining returns the region concurrent with region — another of
// the same composite state's regions, or another of the machine's own — that
// contains state, and nil when state lies outside that region set.
func (e *StateExecutor) siblingRegionContaining(region *ast.StateRegion, state *ast.StateNode) *ast.StateRegion {
	siblings := e.graph.TopRegions
	if owner := e.graph.RegionOwner[region]; owner != nil {
		siblings = e.graph.CompositeStates[owner]
	}
	for _, sibling := range siblings {
		if sibling != region && e.regionContains(sibling, state) {
			return sibling
		}
	}
	return nil
}

// innermostActiveRegion descends from region through the already-active states on
// the path to target, which stay active, and returns the region the move happens in.
func (e *StateExecutor) innermostActiveRegion(region *ast.StateRegion, target *ast.StateNode) *ast.StateRegion {
	for {
		active, ok := e.activeConfig.regionStates[region]
		if !ok || active == target {
			return region
		}
		inner := e.regionUnder(active, target)
		if inner == nil {
			return region
		}
		region = inner
	}
}

// regionUnder returns the orthogonal region of state that contains target, and
// nil when target is not below one of state's own regions.
func (e *StateExecutor) regionUnder(state, target *ast.StateNode) *ast.StateRegion {
	for current := target; current != nil; current = e.graph.ParentState[current] {
		if e.graph.ParentState[current] == state {
			return e.declaringRegion(current)
		}
	}
	return nil
}

// moveBetweenRegions moves sourceRegion's active state — source, or a state
// nested below it — to a target in targetRegion, which is either sourceRegion
// itself or a region concurrent with it. KerML's StateTransitionPerformance orders only
// `guard then transitionLinkSource.exit`, so the state owning the regions is
// neither exited nor re-entered and the regions holding neither endpoint keep
// their active states.
func (e *StateExecutor) moveBetweenRegions(
	sourceRegion, targetRegion *ast.StateRegion,
	source *ast.StateNode,
	trans *lower.Transition,
	target *ast.StateNode,
) error {
	// Entering the target replaces its region's active state, so that region is
	// left only down to the deepest state it keeps active — its own boundary when
	// it shares none with the target. The state owning the region stays active.
	keep := e.getLCA(e.activeConfig.regionStates[targetRegion], target)
	// A transition out of a composite state is external even inside a region: the
	// source is exited and re-entered when it encloses the target.
	if declared, isState := trans.Source.(*ast.StateNode); isState && e.encloses(declared, target) {
		keep = e.graph.ParentState[declared]
	}
	if !e.regionContains(targetRegion, keep) {
		keep = e.graph.RegionOwner[targetRegion]
	}

	if sourceRegion == targetRegion {
		if err := e.exitRegionTo(sourceRegion, keep); err != nil {
			return err
		}
	} else {
		// The source region holds neither the target nor any state below it, so it
		// is left entirely: the transition's source performance exits.
		if err := e.exitRegionTo(sourceRegion, nil); err != nil {
			return err
		}
		if err := e.exitRegionTo(targetRegion, keep); err != nil {
			return err
		}
	}

	if err := e.runEffect(trans); err != nil {
		return err
	}

	enter, branches := e.entryPlan(keep, target)
	// The region's active state is the deepest state on the path to target that the
	// region itself declares, which is a composite state above target when the
	// target is nested inside one.
	leaf := target
	if branch, ok := e.branchesTo(nil, target)[targetRegion]; ok {
		leaf = branch
	}
	// The region's own entry is recorded before entering, so a state entered
	// inside it is not mistaken for the single active state of a simple machine.
	e.activeConfig.regionStates[targetRegion] = leaf
	for _, state := range e.descendantChain(keep, enter) {
		if err := e.enterStateInto(state, branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}
	if _, err := e.enterStartOf(target); err != nil {
		return err
	}

	if err := e.scheduleFromEntered(leaf); err != nil {
		return err
	}
	if err := e.completeIfDone(target); err != nil {
		return fmt.Errorf("complete state machine: %w", err)
	}
	e.recordTransitionTrace(trans, source, target)
	return nil
}

// exitRegionTo exits region's active state and the states between it and stop,
// which stays active; a nil stop leaves the region entirely, up to its own
// boundary. The region is left without an active state.
func (e *StateExecutor) exitRegionTo(region *ast.StateRegion, stop *ast.StateNode) error {
	active, ok := e.activeConfig.regionStates[region]
	if !ok {
		return nil
	}
	delete(e.activeConfig.regionStates, region)
	if stop == nil {
		// The region keeps no active state, so it has none to restore either.
		e.forgetRegionHistory(region)
	}
	for current := active; current != nil && current != stop && e.regionContains(region, current); current = e.graph.ParentState[current] {
		if err := e.exitState(current); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	return nil
}

// leaveRegion takes a transition whose target lies outside the region set its
// source is active in — outside the composite state that owns the regions. The
// whole set is left: every sibling region is exited, recording its configuration
// for history, before the target is entered.
func (e *StateExecutor) leaveRegion(region *ast.StateRegion, trans *lower.Transition, target *ast.StateNode) error {
	source := e.activeConfig.regionStates[region]
	owner := e.graph.RegionOwner[region]
	if owner == nil {
		return e.leaveTopRegions(trans, source, target)
	}

	// The target is outside the composite state: exit it and its ancestors up to
	// the least common ancestor, then enter down to the target. Exiting the state
	// exits its regions' active states, as exiting a KerML StatePerformance ends
	// its subperformances.
	lca := e.getLCA(owner, target)
	for current := owner; current != nil && current != lca; current = e.graph.ParentState[current] {
		// Clear the region current is active in first — a region's active state may
		// be nested below current — or an enclosing state exits current again.
		if declaring := e.enclosingRegion(current); declaring != nil {
			if active, isActive := e.activeConfig.regionStates[declaring]; isActive && e.isBelowOrEqual(active, current) {
				e.recordRegionHistory(declaring, active)
				delete(e.activeConfig.regionStates, declaring)
			}
		}
		if err := e.exitState(current); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	if err := e.runEffect(trans); err != nil {
		return err
	}
	return e.enterOutside(trans, source, lca, target)
}

// leaveTopRegions leaves the machine's own orthogonal regions, which no state
// owns: every region is exited in declaration order and the target — outside
// all of them — is then entered as the machine's single active state.
func (e *StateExecutor) leaveTopRegions(trans *lower.Transition, source, target *ast.StateNode) error {
	for _, region := range e.graph.TopRegions {
		active, ok := e.activeConfig.regionStates[region]
		if !ok {
			continue
		}
		delete(e.activeConfig.regionStates, region)
		leaving := make([]*ast.StateNode, 0)
		for current := active; current != nil; current = e.graph.ParentState[current] {
			leaving = append(leaving, current)
		}
		if err := e.exitStates(leaving); err != nil {
			return err
		}
	}
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.activeConfig.simpleState = nil

	if err := e.runEffect(trans); err != nil {
		return err
	}

	return e.enterOutside(trans, source, nil, target)
}

// enterOutside enters target below lca after the region set the transition left
// has been torn down, and finishes the move.
func (e *StateExecutor) enterOutside(trans *lower.Transition, source, lca, target *ast.StateNode) error {
	enter, branches := e.entryPlan(lca, target)
	for _, state := range e.descendantChain(lca, enter) {
		if err := e.enterStateInto(state, branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}
	deepest, err := e.enterStartOf(target)
	if err != nil {
		return err
	}

	// Record the entered path: the deepest entered state of every orthogonal
	// region on it becomes that region's active state, and a target inside none
	// of them becomes the machine's single active state.
	onPath := e.branchesTo(nil, target)
	for region, leaf := range onPath {
		e.activeConfig.regionStates[region] = leaf
	}
	if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = deepest
	}

	e.stateStack = e.rootToLeaf(deepest)
	if err := e.scheduleFromEntered(enter); err != nil {
		return err
	}
	if err := e.completeIfDone(target); err != nil {
		return fmt.Errorf("complete state machine: %w", err)
	}
	e.recordTransitionTrace(trans, source, target)
	return nil
}

// rootToLeaf returns state's ancestors and state itself, outermost first.
func (e *StateExecutor) rootToLeaf(state *ast.StateNode) []*ast.StateNode {
	chain := e.getParentChain(state)
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// recordTransitionTrace records one transition in the trace, naming the state
// left and the state entered.
func (e *StateExecutor) recordTransitionTrace(trans *lower.Transition, source, target *ast.StateNode) {
	if e.trace() == nil {
		return
	}
	from := ""
	if source != nil {
		from = source.Name
	}
	e.trace().RecordStateTransition(from, target.Name, triggerName(trans.Trigger))
}

// orderedActiveRegions returns the active orthogonal regions in declaration
// order. The order regions react to a broadcast event in is observable, so it
// must not depend on map iteration order.
func (e *StateExecutor) orderedActiveRegions() []*ast.StateRegion {
	regions := make([]*ast.StateRegion, 0, len(e.activeConfig.regionStates))
	for _, region := range e.graph.TopRegions {
		if _, active := e.activeConfig.regionStates[region]; active {
			regions = append(regions, region)
		}
	}
	for _, state := range e.graph.States {
		for _, region := range e.graph.CompositeStates[state] {
			if _, active := e.activeConfig.regionStates[region]; active {
				regions = append(regions, region)
			}
		}
	}
	return regions
}
