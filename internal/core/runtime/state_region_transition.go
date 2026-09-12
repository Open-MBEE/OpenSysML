package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

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

// fireTransitionInRegion fires a transition whose source is the active state of
// an orthogonal region. A target inside the same region moves only that region;
// a target outside it leaves the whole region set, which is what makes a
// transition through a choice or junction reachable from inside a region.
func (e *StateExecutor) fireTransitionInRegion(region *ast.StateRegion, trans *lower.Transition, r route) (bool, error) {
	// Fork, join and history replace the entire active configuration rather than
	// move one region, so they are fired whole.
	if isSynchronizationTarget(trans.Target) {
		return e.fireTransition(trans, r)
	}

	pass, err := e.passesGuard(trans)
	if err != nil || !pass {
		return false, err
	}
	e.transitionDecided()

	if !r.settled() {
		return false, fmt.Errorf("transition out of region %s has no target state", region.Name)
	}

	source := e.activeConfig.regionStates[region]
	return true, e.travel(r, source,
		func(target *ast.StateNode) []*ast.StateNode { return e.exitedInRegion(region, trans, target) },
		func(effects []lower.StateBehavior, target *ast.StateNode) error {
			return e.moveInRegion(region, source, trans, effects, target)
		})
}

// moveInRegion finishes a move out of region's active state source: within the
// region, across to a concurrent one, or out of the whole region set.
func (e *StateExecutor) moveInRegion(region *ast.StateRegion, source *ast.StateNode, trans *lower.Transition, effects []lower.StateBehavior, target *ast.StateNode) error {
	sourceRegion, targetRegion := e.regionMove(region, target)
	if targetRegion == nil {
		return e.leaveRegion(region, trans, effects, target)
	}
	return e.moveBetweenRegions(sourceRegion, targetRegion, source, trans, effects, target)
}

// regionMove is the region a transition out of region's active state leaves and
// the one it moves within to reach target: region itself for a target inside it,
// the concurrent pair otherwise, and nil when target lies outside the region set.
func (e *StateExecutor) regionMove(region *ast.StateRegion, target *ast.StateNode) (*ast.StateRegion, *ast.StateRegion) {
	if e.regionContains(region, target) {
		return region, region
	}
	if exit, sibling := e.concurrentRegionsFor(region, target); sibling != nil {
		return exit, e.innermostActiveRegion(sibling, target)
	}
	return nil, nil
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
	effects []lower.StateBehavior,
	target *ast.StateNode,
) error {
	keep := e.regionKeep(targetRegion, trans, target)

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

	if err := e.runBehaviors(effects); err != nil {
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
	// The target's own entry transitions may start it in a nested state, which
	// then is the deepest state the region keeps active.
	deepest, err := e.enterStartOf(target)
	if err != nil {
		return err
	}
	if branch, ok := e.branchesTo(nil, deepest)[targetRegion]; ok {
		leaf = branch
		e.activeConfig.regionStates[targetRegion] = leaf
	}

	if err := e.scheduleFromEntered(leaf); err != nil {
		return err
	}
	if err := e.completeIfDone(deepest); err != nil {
		return fmt.Errorf("complete state machine: %w", err)
	}
	e.recordTransitionTrace(trans, source, target)
	return nil
}

// regionKeep is the deepest state targetRegion keeps active when a transition
// enters target in it: what its active state shares with the target, the parent
// of a source enclosing the target (an external transition even inside a region),
// or the region's owner when nothing inside it is shared.
func (e *StateExecutor) regionKeep(targetRegion *ast.StateRegion, trans *lower.Transition, target *ast.StateNode) *ast.StateNode {
	keep := e.getLCA(e.activeConfig.regionStates[targetRegion], target)
	if declared, isState := trans.Source.(*ast.StateNode); isState && e.encloses(declared, target) {
		keep = e.graph.ParentState[declared]
	}
	if !e.regionContains(targetRegion, keep) {
		keep = e.graph.RegionOwner[targetRegion]
	}
	return keep
}

// regionExitPath lists the states exitRegionTo exits, innermost first: region's
// active state and its ancestors up to stop, within the region.
func (e *StateExecutor) regionExitPath(region *ast.StateRegion, stop *ast.StateNode) []*ast.StateNode {
	active, ok := e.activeConfig.regionStates[region]
	if !ok {
		return nil
	}
	return e.exitPath(active, stop, region)
}

// exitRegionTo exits region's active state and the states between it and stop,
// which stays active; a nil stop leaves the region entirely, up to its own
// boundary. The region is left without an active state.
func (e *StateExecutor) exitRegionTo(region *ast.StateRegion, stop *ast.StateNode) error {
	if _, ok := e.activeConfig.regionStates[region]; !ok {
		return nil
	}
	path := e.regionExitPath(region, stop)
	delete(e.activeConfig.regionStates, region)
	if stop == nil {
		// The region keeps no active state, so it has none to restore either.
		e.forgetRegionHistory(region)
	}
	for _, current := range path {
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
func (e *StateExecutor) leaveRegion(region *ast.StateRegion, trans *lower.Transition, effects []lower.StateBehavior, target *ast.StateNode) error {
	source := e.activeConfig.regionStates[region]
	owner := e.graph.RegionOwner[region]
	if owner == nil {
		return e.leaveTopRegions(trans, effects, source, target)
	}

	// The target is outside the composite state: exit it and its ancestors up to
	// the least common ancestor, then enter down to the target. Exiting the state
	// exits its regions' active states, as exiting a KerML StatePerformance ends
	// its subperformances.
	lca := e.getLCA(owner, target)
	for _, current := range e.exitPath(owner, lca, nil) {
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
	if err := e.runBehaviors(effects); err != nil {
		return err
	}
	return e.enterOutside(trans, source, lca, target)
}

// leaveTopRegions leaves the machine's own orthogonal regions, which no state
// owns: every region is exited in declaration order and the target — outside
// all of them — is then entered as the machine's single active state.
func (e *StateExecutor) leaveTopRegions(trans *lower.Transition, effects []lower.StateBehavior, source, target *ast.StateNode) error {
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

	if err := e.runBehaviors(effects); err != nil {
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
	onPath := e.branchesTo(nil, deepest)
	for region, leaf := range onPath {
		e.activeConfig.regionStates[region] = leaf
	}
	if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = deepest
	}

	e.stateStack = e.rootToLeaf(deepest)
	// Scheduling starts from the deepest state entered when the path descends
	// through no orthogonal regions; otherwise their recorded leaves are used.
	scheduleFrom := enter
	if enter == target {
		scheduleFrom = deepest
	}
	if err := e.scheduleFromEntered(scheduleFrom); err != nil {
		return err
	}
	if err := e.completeIfDone(deepest); err != nil {
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
