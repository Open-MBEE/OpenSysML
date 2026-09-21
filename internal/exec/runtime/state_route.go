package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// transientPseudostate reports whether a pseudostate merely routes a transition
// onwards — choice and junction — as opposed to fork, join and history, which
// rewrite the whole active configuration.
func transientPseudostate(kind ast.PseudostateKind) bool {
	switch kind {
	case ast.PseudostateChoice, ast.PseudostateJunction:
		return true
	}
	return false
}

// route is a compound transition's path as far as it is settled: the segments to
// run, the transition first, ending at a state, open at a choice whose guards
// are read only once those segments' effects have run, or open at a junction
// several of whose branches its guards enabled, drawn among once the transition
// fires. None is set for a fork, a waiting join or a history restoring.
type route struct {
	segments []*lower.Transition
	target   *ast.StateNode
	choice   *ast.PseudostateNode
	draw     *junctionDraw
	// crossed are the pseudostates passed, so a path back into one is a cycle.
	crossed []*ast.PseudostateNode
	// notes is what settling the route noted — a branch guard it could not read,
	// a junction drawn among several enabled — recorded once the transition is taken.
	notes []RunNote
	// terminate is the terminate action usage the route ends at, which ends the
	// machine's performance in place of entering a state (SysML v2 §7.18.3).
	terminate *ast.Usage
}

// junctionDraw is a junction the route reached with several branches enabled, as
// their guards read when the transition was selected, and the route on from each
// as settled then; the policy draws among them only once the transition is
// committed to fire, so a transition another region's reaction leaves behind
// draws nothing.
type junctionDraw struct {
	at       *ast.PseudostateNode
	outgoing []*lower.Transition
	enabled  []int
	// beyond is the route on from each enabled branch, in enabled's order.
	beyond []branchBeyond
}

// branchBeyond is the route on from one enabled branch, or why it could not be
// settled — a dead end only the run that draws the branch runs into.
type branchBeyond struct {
	route route
	err   error
}

// settled reports whether the route has somewhere to move to.
func (r route) settled() bool {
	return r.target != nil || r.choice != nil || r.draw != nil || r.terminate != nil
}

// routeEffect is one effect of a compound transition and the state declaring the
// pseudostate its segment leaves; nil for a segment out of a state or the machine's body.
type routeEffect struct {
	behavior lower.StateBehavior
	within   *ast.StateNode
	segment  *lower.Transition
}

// effects are the behaviors the route's segments perform, in path order, each
// with the state enclosing it.
func (r route) effects(g *lower.StateGraph) []routeEffect {
	var effects []routeEffect
	for _, seg := range r.segments {
		var within *ast.StateNode
		if ps, isPseudostate := seg.Source.(*ast.PseudostateNode); isPseudostate {
			within = g.PseudostateOwner[ps]
		}
		for _, behavior := range seg.Effect {
			effects = append(effects, routeEffect{behavior: behavior, within: within, segment: seg})
		}
	}
	return effects
}

// resolveRoute settles a transition's route before anything moves: its target, or
// on through junctions or a join, up to the first choice; a history stays unsettled.
func (e *StateExecutor) resolveRoute(trans *lower.Transition) (route, error) {
	r := route{segments: []*lower.Transition{trans}}
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		r.target = target
		return r, nil
	case *ast.PseudostateNode:
		switch target.Kind {
		case ast.PseudostateFork, ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
			return r, nil
		case ast.PseudostateJoin:
			sources, err := e.joinSources(target)
			if err != nil {
				return route{}, err
			}
			if !e.allActive(sources) {
				return r, nil
			}
		}
		r, err := e.followOut(target, r)
		if err != nil {
			return route{}, fmt.Errorf("evaluate pseudostate: %w", err)
		}
		return r, nil
	case *ast.Usage:
		if lower.IsTerminateUsage(target) {
			r.terminate = target
			return r, nil
		}
	}
	return route{}, fmt.Errorf("transition target must be a state, pseudostate or terminate action, got %T", trans.Target)
}

// followOut goes on from a pseudostate the route has reached: a choice leaves the
// route open there; out of any other — junction, join or history — the branches
// are read against the data as they stand, the one enabled is taken and several
// leave the route open at the draw among them. Exactly one succession is taken,
// as KerML `DecisionPerformance::outgoingHBLink: HappensBefore[1]` requires.
// A route that cannot be settled is returned as far as it got, with its notes.
func (e *StateExecutor) followOut(ps *ast.PseudostateNode, r route) (route, error) {
	if slices.Contains(r.crossed, ps) {
		return r, fmt.Errorf("%s %s: outgoing transitions form a cycle between pseudostates", ps.Kind, ps.Name)
	}
	r.crossed = append(r.crossed, ps)
	if ps.Kind == ast.PseudostateChoice {
		r.choice = ps
		return r, nil
	}
	outgoing := e.graph.Transitions[ps]
	if len(outgoing) == 0 {
		return r, fmt.Errorf("%s %s has no outgoing transitions", ps.Kind, ps.Name)
	}
	enabled, notes, err := e.enabledBranches(ps, outgoing)
	if err != nil {
		return r, err
	}
	r.notes = append(r.notes, notes...)
	if len(enabled) == 0 {
		return r, fmt.Errorf("%s %s: no guard evaluated to true", ps.Kind, ps.Name)
	}
	if len(enabled) > 1 {
		draw := &junctionDraw{at: ps, outgoing: outgoing, enabled: enabled, beyond: make([]branchBeyond, len(enabled))}
		for i, pos := range enabled {
			beyond, err := e.follow(ps, outgoing[pos], route{crossed: slices.Clone(r.crossed)})
			draw.beyond[i] = branchBeyond{route: beyond, err: err}
		}
		r.draw = draw
		return r, nil
	}
	return e.follow(ps, outgoing[enabled[0]], r)
}

// settleDraws makes the draws the route is open at, in turn, once the transition
// is committed to fire and nothing has moved yet: the policy draws among the
// enabled branches, as a choice point among the route's notes, and the route goes
// on along the one drawn as it was settled when the transition was selected, no
// guard beyond read again; a branch that could not be settled fails the run
// that draws it, the draw and what the branch noted on its way among the notes.
// A draw the witness refuses is the refusal, and the route is left where it was.
func (e *StateExecutor) settleDraws(r route) (route, error) {
	for r.draw != nil {
		draw := r.draw
		r.draw = nil
		pick, err := e.pickBranch(draw.at, draw.outgoing, draw.enabled, func(n RunNote) { r.notes = append(r.notes, n) })
		if err != nil {
			return route{}, err
		}
		beyond := draw.beyond[pick]
		if beyond.err != nil {
			r.notes = append(r.notes, beyond.route.notes...)
			return r, fmt.Errorf("evaluate pseudostate: %w", beyond.err)
		}
		r = r.onward(beyond.route)
	}
	return r, nil
}

// onward is the route continued along beyond, settled from the pseudostate this
// route is open at: its segments follow, and it ends where beyond does.
func (r route) onward(beyond route) route {
	r.segments = append(r.segments, beyond.segments...)
	r.target, r.choice, r.draw, r.terminate = beyond.target, beyond.choice, beyond.draw, beyond.terminate
	r.crossed = beyond.crossed
	r.notes = append(r.notes, beyond.notes...)
	return r
}

// follow takes a segment out of the pseudostate from and goes on from its target.
func (e *StateExecutor) follow(from *ast.PseudostateNode, seg *lower.Transition, r route) (route, error) {
	r.segments = append(r.segments, seg)
	switch target := seg.Target.(type) {
	case *ast.StateNode:
		r.target = target
		return r, nil
	case *ast.PseudostateNode:
		if !transientPseudostate(target.Kind) {
			return r, fmt.Errorf("%s %s: a transition into %s %s is not supported", from.Kind, from.Name, target.Kind, target.Name)
		}
		return e.followOut(target, r)
	case *ast.Usage:
		if lower.IsTerminateUsage(target) {
			r.terminate = target
			return r, nil
		}
	}
	return r, fmt.Errorf("%s %s: target must be a state, pseudostate or terminate action, got %T", from.Kind, from.Name, seg.Target)
}

// resolveChoice reads the guards of the choice the route is open at against the
// data as it now stands and goes on from the branch taken: the policy draws among
// several enabled, as a recorded choice point; none enabled is the typed error.
func (e *StateExecutor) resolveChoice(r route) (route, error) {
	choice := r.choice
	outgoing := e.graph.Transitions[choice]
	enabled, notes, err := e.enabledBranches(choice, outgoing)
	if err != nil {
		return route{}, err
	}
	e.noteAll(notes)
	if len(enabled) == 0 {
		return route{}, fmt.Errorf("%w: choice %s: no guard evaluated to true", ErrChoiceWithoutBranch, choice.Name)
	}
	pick, err := e.pickBranch(choice, outgoing, enabled, e.ctx.note)
	if err != nil {
		return route{}, err
	}
	// A branch past the first was only probed; its guard's final reading is made now.
	if pick > 0 {
		if _, err := e.passesGuard(outgoing[enabled[pick]]); err != nil {
			return route{}, fmt.Errorf("choice %s: %w", choice.Name, err)
		}
	}
	// The route past a choice is followed while firing, its draws made at once, so
	// what it notes is noted now.
	r, err = e.follow(choice, outgoing[enabled[pick]], route{crossed: r.crossed})
	if err == nil {
		r, err = e.settleDraws(r)
	}
	e.noteAll(r.notes)
	r.notes = nil
	return r, err
}

// enabledBranches reads the guards of the branches out of ps against the data as
// it stands: the positions of those that hold, else of the unguarded ones, the
// else branches. Once a branch holds the rest are probed only to report the
// choice; one with no result is not a branch, and is returned as a note.
func (e *StateExecutor) enabledBranches(ps *ast.PseudostateNode, outgoing []*lower.Transition) ([]int, []RunNote, error) {
	var enabled, unguarded []int
	var notes []RunNote
	for i, trans := range outgoing {
		if trans.Guard == nil {
			unguarded = append(unguarded, i)
			continue
		}
		var pass bool
		if len(enabled) > 0 {
			var unevaluable *UnevaluableGuard
			if pass, unevaluable = e.probeBranch(ps, outgoing, i); unevaluable != nil {
				notes = append(notes, *unevaluable)
			}
		} else {
			var err error
			if pass, err = e.passesGuard(trans); err != nil {
				return nil, nil, fmt.Errorf("%s %s: %w", ps.Kind, ps.Name, err)
			}
		}
		if pass {
			enabled = append(enabled, i)
		}
	}
	if len(enabled) == 0 {
		enabled = unguarded
	}
	return enabled, notes, nil
}

// pickBranch is the index into enabled of the branch out of ps taken: the only
// one, or the one the policy draws among several, handed to note as the choice
// point taken; a draw the witness refuses is the refusal. The guards are not
// read again: a junction's were read once, when its transition was selected.
func (e *StateExecutor) pickBranch(ps *ast.PseudostateNode, outgoing []*lower.Transition, enabled []int, note func(RunNote)) (int, error) {
	// Weights are validated for a lone enabled branch too, even though it
	// records no choice point and is taken with probability 1.
	weights, err := e.transitionWeights(ps, outgoing, enabled, nil)
	if err != nil {
		return 0, err
	}
	point, ok := e.branchPoint(ps, outgoing, enabled)
	if !ok {
		return 0, nil
	}
	var pick int
	if weights != nil {
		point.Weights = weights
		if err := e.ctx.scheduling().chooseWeighted(&point, nil); err != nil {
			return 0, err
		}
		pick = point.Taken
	} else {
		pick = e.ctx.scheduling().choose(point, nil)
	}
	if err := e.ctx.scheduling().refusal(); err != nil {
		return 0, err
	}
	point.Taken = pick
	point.File, point.Span = e.transitionLocation(ps, outgoing[enabled[pick]])
	note(point)
	return pick, nil
}

// probeBranch reads whether the branch at position i out of ps holds once
// another already does, as a probe the context undoes whole; one that cannot be
// evaluated is not enabled and is returned as the note to record.
func (e *StateExecutor) probeBranch(ps *ast.PseudostateNode, outgoing []*lower.Transition, i int) (bool, *UnevaluableGuard) {
	var pass bool
	var err error
	e.preview(func() { pass, err = e.passesGuard(outgoing[i]) })
	if err != nil {
		file, _ := e.transitionLocation(ps, outgoing[i])
		return false, &UnevaluableGuard{
			Where:       pseudostateWhere(ps),
			Alternative: transitionName(outgoing, i),
			Reason:      err.Error(),
			File:        file,
			Span:        outgoing[i].Guard.Span(),
		}
	}
	return pass, nil
}

// branchPoint is the branches out of ps enabled, at their declared positions, as
// a choice point not yet taken; there is none under two.
func (e *StateExecutor) branchPoint(ps *ast.PseudostateNode, outgoing []*lower.Transition, enabled []int) (ChoicePoint, bool) {
	if len(enabled) < 2 {
		return ChoicePoint{}, false
	}
	alts := make([]string, len(enabled))
	for i, pos := range enabled {
		alts[i] = transitionName(outgoing, pos)
	}
	return ChoicePoint{
		Kind:         ChoiceTransition,
		Where:        pseudostateWhere(ps),
		Alternatives: alts,
	}, true
}

// pseudostateWhere names a pseudostate for a note, `choice pick` or `junction split`.
func pseudostateWhere(ps *ast.PseudostateNode) string {
	return fmt.Sprintf("%s %s", ps.Kind, ps.Name)
}

// reachable lists the states, and the terminate actions, the route open at a
// choice or a draw can end at: along the route settled beyond each branch of the
// junction enabled (one that could not be settled ends nowhere), or through every
// branch of the choice and whatever pseudostates lie beyond, each once.
func (e *StateExecutor) reachable(r route) (states []*ast.StateNode, stops []*ast.Usage, err error) {
	reach := &reachSet{graph: e.graph, seen: make(map[*ast.PseudostateNode]bool)}
	for _, ps := range r.crossed {
		reach.seen[ps] = true
	}
	err = reach.settled(r)
	return reach.states, reach.stops, err
}

// reachSet gathers the states and terminate actions a route can end at, each
// once, crossing each transient pseudostate once.
type reachSet struct {
	graph  *lower.StateGraph
	seen   map[*ast.PseudostateNode]bool
	states []*ast.StateNode
	stops  []*ast.Usage
}

func (s *reachSet) add(target *ast.StateNode) {
	if !slices.Contains(s.states, target) {
		s.states = append(s.states, target)
	}
}

func (s *reachSet) stop(target *ast.Usage) {
	if !slices.Contains(s.stops, target) {
		s.stops = append(s.stops, target)
	}
}

// visit follows every branch out of ps and whatever pseudostates lie beyond.
func (s *reachSet) visit(ps *ast.PseudostateNode, branches []*lower.Transition) error {
	for _, branch := range branches {
		switch target := branch.Target.(type) {
		case *ast.StateNode:
			s.add(target)
		case *ast.PseudostateNode:
			if !transientPseudostate(target.Kind) {
				return fmt.Errorf("%s %s: a transition into %s %s is not supported", ps.Kind, ps.Name, target.Kind, target.Name)
			}
			if s.seen[target] {
				continue
			}
			s.seen[target] = true
			if err := s.visit(target, s.graph.Transitions[target]); err != nil {
				return err
			}
		case *ast.Usage:
			if !lower.IsTerminateUsage(target) {
				return fmt.Errorf("%s %s: target must be a state, pseudostate or terminate action, got %T", ps.Kind, ps.Name, branch.Target)
			}
			s.stop(target)
		default:
			return fmt.Errorf("%s %s: target must be a state, pseudostate or terminate action, got %T", ps.Kind, ps.Name, branch.Target)
		}
	}
	return nil
}

// settled follows the route where it is settled, and every branch of the
// choice where it is not.
func (s *reachSet) settled(r route) error {
	if r.target != nil {
		s.add(r.target)
		return nil
	}
	if r.terminate != nil {
		s.stop(r.terminate)
		return nil
	}
	if r.draw != nil {
		for _, beyond := range r.draw.beyond {
			if beyond.err != nil {
				continue
			}
			if err := s.settled(beyond.route); err != nil {
				return err
			}
		}
		return nil
	}
	return s.visit(r.choice, s.graph.Transitions[r.choice])
}

// terminateBoundary is the state a move from `from` into stop's owner stays inside
// of, as moveBoundary finds it for a state target: nil for the machine's body.
func (e *StateExecutor) terminateBoundary(from *ast.StateNode, trans *lower.Transition, stop *ast.Usage) *ast.StateNode {
	owner := e.graph.TerminateOwner[stop]
	if source, isState := trans.Source.(*ast.StateNode); isState && e.encloses(source, owner) {
		return e.graph.ParentState[source]
	}
	if owner == nil || from == nil {
		return nil
	}
	return e.getLCA(from, owner)
}

// terminateExits lists the states a move from `from` to stop exits, innermost
// first: those below the boundary, as a move to a state in stop's body would.
func (e *StateExecutor) terminateExits(from *ast.StateNode, trans *lower.Transition, stop *ast.Usage) []*ast.StateNode {
	return e.exitPath(from, e.terminateBoundary(from, trans, stop), nil)
}

// terminateEntries lists the states a move from `from` to stop enters, outermost
// first: the chain from the boundary down to the state declaring stop.
func (e *StateExecutor) terminateEntries(from *ast.StateNode, trans *lower.Transition, stop *ast.Usage) []*ast.StateNode {
	return e.descendantChain(e.terminateBoundary(from, trans, stop), e.graph.TerminateOwner[stop])
}

// exitPlan lists the states a move to target exits, against the configuration
// as it stands.
type exitPlan func(target *ast.StateNode) []*ast.StateNode

// entryPlan lists the states a move to target enters, outermost first, against
// the configuration as it stands.
type entryPlan func(target *ast.StateNode) []*ast.StateNode

// routeEnds are the moves to the states and terminate actions the branches
// beyond a choice may end at: the exits and entries each makes.
type routeEnds struct {
	exits   [][]*ast.StateNode
	entries [][]*ast.StateNode
}

// endsOf plans the move to every end the route open at a choice may reach.
func (e *StateExecutor) endsOf(r route, trans *lower.Transition, from *ast.StateNode, exits exitPlan, enters entryPlan) (routeEnds, error) {
	targets, stops, err := e.reachable(r)
	if err != nil {
		return routeEnds{}, err
	}
	var ends routeEnds
	for _, target := range targets {
		ends.exits = append(ends.exits, e.expandExits(exits(target)))
		ends.entries = append(ends.entries, enters(target))
	}
	for _, stop := range stops {
		ends.exits = append(ends.exits, e.expandExits(e.terminateExits(from, trans, stop)))
		ends.entries = append(ends.entries, e.terminateEntries(from, trans, stop))
	}
	return ends, nil
}

// certainExits lists the states a move to every one of the ends exits — in a
// sibling region as well as above the source — in the order the first end's
// move leaves them, so they can be left before the branch is known.
func (ends routeEnds) certainExits() []*ast.StateNode {
	return certainStates(ends.exits)
}

// certainEntries lists the states a move to every one of the ends enters, in
// the order the first end's move enters them, before the branch is known.
func (ends routeEnds) certainEntries() []*ast.StateNode {
	return certainStates(ends.entries)
}

// certainStates lists the states every list holds, in the first list's order.
func certainStates(lists [][]*ast.StateNode) []*ast.StateNode {
	if len(lists) == 0 {
		return nil
	}
	certain := slices.Clone(lists[0])
	for _, other := range lists[1:] {
		certain = slices.DeleteFunc(certain, func(state *ast.StateNode) bool {
			return !slices.Contains(other, state)
		})
	}
	return certain
}

// runEffects performs a compound transition's effects in path order, activating the
// chain down to the state enclosing each first: a segment is a performance of its owner.
// A `terminate` ending a step of a transition body ends the body's later steps (endedBefore).
func (e *StateExecutor) runEffects(effects []routeEffect, chain []*ast.StateNode) error {
	var ended []ast.Node
	for i, effect := range effects {
		if e.endedBefore(ended, effect.behavior) {
			continue
		}
		if upto := e.enclosingIndex(chain, effect.within); upto >= 0 {
			if err := e.enterAhead(chain[:upto+1]); err != nil {
				return err
			}
		}
		// A segment's effects are one unit of the firing, run in their order.
		if i == 0 || effects[i-1].segment != effect.segment {
			if _, err := e.unit(ChoiceRegionOrder, unitHead{label: e.effectLabel(effect.segment), at: effect.segment.Decl}); err != nil {
				return err
			}
		}
		terminated, err := e.executeBehavior(effect.behavior)
		if err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
		if terminated {
			ended = append(ended, effect.behavior.Block)
		}
	}
	return nil
}

// enclosingIndex is where the state enclosing an effect, or the parallel state
// whose region it stands for, lies in chain; -1 when the chain never enters it.
func (e *StateExecutor) enclosingIndex(chain []*ast.StateNode, state *ast.StateNode) int {
	if state == nil || state == e.graph.Machine {
		return -1
	}
	if i := slices.Index(chain, state); i >= 0 {
		return i
	}
	if region := e.graph.HiddenRegionOf[state]; region != nil {
		return slices.Index(chain, e.graph.RegionOwner[region])
	}
	return -1
}

// enterOwnerOf activates the chain down to the state declaring ps, when the move
// enters it: the guards of a choice are read once its owner is entered.
func (e *StateExecutor) enterOwnerOf(ps *ast.PseudostateNode, chain []*ast.StateNode) error {
	upto := e.enclosingIndex(chain, e.graph.PseudostateOwner[ps])
	if upto < 0 {
		return nil
	}
	return e.enterAhead(chain[:upto+1])
}

// enterAhead activates the states of chain not yet activated, outermost first; the
// move entering them later finds them activated and goes on with their regions.
func (e *StateExecutor) enterAhead(chain []*ast.StateNode) error {
	simple := e.activeConfig.simpleState
	defer func() { e.activeConfig.simpleState = simple }()
	for _, state := range chain {
		if _, ahead := e.enteredAhead[state]; ahead {
			continue
		}
		if e.entryIsUnit(state) {
			if _, err := e.unit(ChoiceEntryOrder, e.entryHead(state, false)); err != nil {
				return err
			}
		}
		if err := e.activateState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
		if e.enteredAhead == nil {
			e.enteredAhead = make(map[*ast.StateNode]bool)
		}
		e.enteredAhead[state] = true
	}
	return nil
}

// activatedAhead reports, and takes, an activation the move already made ahead.
func (e *StateExecutor) activatedAhead(state *ast.StateNode) bool {
	if !e.enteredAhead[state] {
		return false
	}
	e.enteredAhead[state] = false
	return true
}

// entriesSettled reports a move that never entered a state activated ahead of it.
func (e *StateExecutor) entriesSettled() error {
	for state, waiting := range e.enteredAhead {
		if waiting {
			return fmt.Errorf("state %s was entered ahead of the transition but the transition does not enter it", state.Name)
		}
	}
	return nil
}

// expandExits lists the states an exit list leaves, innermost first: every active
// state below each listed state, which exiting a composite state exits with it,
// then the state itself.
func (e *StateExecutor) expandExits(listed []*ast.StateNode) []*ast.StateNode {
	var left []*ast.StateNode
	for _, state := range listed {
		for _, active := range e.activeLeavesBelow(state) {
			for _, below := range e.exitPath(active, state, nil) {
				if !slices.Contains(left, below) {
					left = append(left, below)
				}
			}
		}
		if !slices.Contains(left, state) {
			left = append(left, state)
		}
	}
	return left
}

// exitAhead exits the states a compound transition leaves whichever branch it
// takes, keeping the configuration as it was so the move finishing it plans its
// exits from it and finds these states left already.
func (e *StateExecutor) exitAhead(states []*ast.StateNode) error {
	if len(states) == 0 {
		return nil
	}
	simple, regions := e.activeConfig.simpleState, e.activeConfig.regionStates
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode, len(regions))
	for region, active := range regions {
		e.activeConfig.regionStates[region] = active
	}
	if e.leftAhead == nil {
		e.leftAhead = make(map[*ast.StateNode]bool)
	}
	e.exitingAhead = true
	err := e.exitStates(states)
	e.exitingAhead = false
	e.activeConfig.simpleState, e.activeConfig.regionStates = simple, regions
	return err
}

// travel takes a compound transition along r as one move: at each choice it leaves what every
// branch leaves, runs the effects into it and reads its guards; move finishes the settled rest.
func (e *StateExecutor) travel(trans *lower.Transition, from *ast.StateNode, r route, exits exitPlan, enters entryPlan, move func([]routeEffect, *ast.StateNode) error) error {
	savedLeft, savedEntered := e.leftAhead, e.enteredAhead
	e.leftAhead, e.enteredAhead = nil, nil
	defer func() { e.leftAhead, e.enteredAhead = savedLeft, savedEntered }()
	if err := e.moveWhole(func() error { return e.travelResolving(trans, from, r, exits, enters, move) }); err != nil {
		return err
	}
	return e.entriesSettled()
}

// moveWhole makes move as one compound transition. Only a replay refuses a move,
// at a draw the draws, exits, effects and entries ahead of it have been made for;
// a refused move is undone whole, the moves nested in it with it.
func (e *StateExecutor) moveWhole(move func() error) error {
	if !e.ctx.scheduling().replaying() {
		return move()
	}
	mark := e.markMove()
	err := move()
	if e.ctx.scheduling().refusal() != nil {
		mark.undo()
	} else {
		mark.keep()
	}
	return err
}

// travelResolving is travel's course: the draw the route is open at made, then
// each choice on the way resolved once the exits every branch makes and the
// effects into it are done. trans is the compound transition's first segment and
// from the state the move leaves, for the trace.
func (e *StateExecutor) travelResolving(trans *lower.Transition, from *ast.StateNode, r route, exits exitPlan, enters entryPlan, move func([]routeEffect, *ast.StateNode) error) error {
	if r.draw != nil {
		var err error
		r, err = e.settleDraws(r)
		e.noteAll(r.notes)
		r.notes = nil
		if err != nil {
			return err
		}
	}
	for r.choice != nil {
		ends, err := e.endsOf(r, trans, from, exits, enters)
		if err != nil {
			return err
		}
		certainExits, certainEntries := ends.certainExits(), ends.certainEntries()
		if _, err := e.leaveAlong(r, certainExits, certainEntries, nil); err != nil {
			return err
		}
		if err := e.exitAhead(certainExits); err != nil {
			return err
		}
		if err := e.enterOwnerOf(r.choice, certainEntries); err != nil {
			return err
		}
		e.noteFired(r.segments...)
		if r, err = e.resolveChoice(r); err != nil {
			return err
		}
	}
	var leaving, entering []*ast.StateNode
	if r.terminate != nil {
		leaving = e.expandExits(e.terminateExits(from, trans, r.terminate))
		entering = e.terminateEntries(from, trans, r.terminate)
	} else {
		leaving, entering = e.expandExits(exits(r.target)), enters(r.target)
	}
	effects, err := e.leaveAlong(r, leaving, entering, r.segments[len(r.segments)-1])
	if err != nil {
		return err
	}
	e.noteFired(r.segments...)
	if r.terminate != nil {
		return e.terminateAlong(trans, from, r, effects)
	}
	return move(effects, r.target)
}

// leaveAlong runs the segments before upto, exiting what each has left before its effects
// (UML 14.2.3.8.4), and returns the effects from upto on, which run after the move's exits.
func (e *StateExecutor) leaveAlong(r route, leaving, entering []*ast.StateNode, upto *lower.Transition) ([]routeEffect, error) {
	effects := r.effects(e.graph)
	var left []*ast.StateNode
	for _, seg := range r.segments {
		if seg == upto {
			break
		}
		left = append(left, e.leftBySegment(leaving, seg)...)
		n := 0
		for n < len(effects) && effects[n].segment == seg {
			n++
		}
		if n == 0 {
			continue
		}
		if err := e.exitAhead(left); err != nil {
			return nil, err
		}
		left = nil
		if err := e.runEffects(effects[:n], entering); err != nil {
			return nil, err
		}
		effects = effects[n:]
	}
	return effects, nil
}

// leftBySegment lists, among the states the move leaves, those seg leaves: the ones from its
// source up to its boundary and their contents; from the machine's body, all still to exit.
func (e *StateExecutor) leftBySegment(leaving []*ast.StateNode, seg *lower.Transition) []*ast.StateNode {
	source := e.vertexState(seg.Source)
	if source == nil {
		return leaving
	}
	boundary := e.segmentBoundary(seg)
	if boundary != nil && e.isBelowOrEqual(boundary, source) {
		return nil
	}
	top := source
	for e.graph.ParentState[top] != boundary && e.graph.ParentState[top] != nil {
		top = e.graph.ParentState[top]
	}
	var left []*ast.StateNode
	for _, state := range leaving {
		if e.isBelowOrEqual(state, top) {
			left = append(left, state)
		}
	}
	return left
}

// segmentBoundary is the state seg stays inside of: the LCA of its ends (a pseudostate standing
// in its declaring state), or the parent of a source that encloses the target; nil is the body.
func (e *StateExecutor) segmentBoundary(seg *lower.Transition) *ast.StateNode {
	source, target := e.vertexState(seg.Source), e.vertexState(seg.Target)
	if declared, isState := seg.Source.(*ast.StateNode); isState && e.encloses(declared, target) {
		return e.graph.ParentState[declared]
	}
	return e.getLCA(source, target)
}

// vertexState is the state a transition end lies in; nil for the machine's body.
func (e *StateExecutor) vertexState(end ast.Node) *ast.StateNode {
	switch v := end.(type) {
	case *ast.StateNode:
		return v
	case *ast.PseudostateNode:
		return e.graph.PseudostateOwner[v]
	case *ast.Usage:
		return e.graph.TerminateOwner[v]
	}
	return nil
}

// terminateAlong finishes a transition at the terminate action its route reaches
// (SysML v2 §7.18.3): the states the move leaves are exited and the ones down to
// the action's owner entered, as for a state beside it, then the machine ends.
func (e *StateExecutor) terminateAlong(trans *lower.Transition, from *ast.StateNode, r route, effects []routeEffect) error {
	if err := e.exitStates(e.terminateExits(from, trans, r.terminate)); err != nil {
		return err
	}
	return e.terminateAt(trans, StateVertexName(from), r, effects, e.terminateEntries(from, trans, r.terminate))
}

// terminateAt ends the machine's performance at the terminate action r reaches,
// the move's exits done: the effects run entering the chain down to the action's
// owner, then no further state is exited, no exit behavior runs, and the do
// behaviors still under way are abandoned where they stand.
func (e *StateExecutor) terminateAt(trans *lower.Transition, fromName string, r route, effects []routeEffect, entering []*ast.StateNode) error {
	if err := e.runEffects(effects, entering); err != nil {
		return err
	}
	if err := e.enterAhead(entering); err != nil {
		return err
	}
	// The machine ends here: the entries made ahead are the move's own.
	clear(e.enteredAhead)
	return e.terminateMachine(fromName, trans.Trigger, r.terminate)
}

// runBehaviors performs a transition's effects, in order.
func (e *StateExecutor) runBehaviors(effects []lower.StateBehavior) error {
	if err := e.executeBehaviors(effects); err != nil {
		return fmt.Errorf("transition effect: %w", err)
	}
	return nil
}

// mayExit lists the states a compound transition along r from `from` may leave,
// whichever end it can reach; false where the route cannot be followed.
func (e *StateExecutor) mayExit(r route, trans *lower.Transition, from *ast.StateNode, exits exitPlan) ([]*ast.StateNode, bool) {
	if r.target != nil {
		return exits(r.target), true
	}
	if r.terminate != nil {
		return e.terminateExits(from, trans, r.terminate), true
	}
	targets, stops, err := e.reachable(r)
	if err != nil {
		return nil, false
	}
	var states []*ast.StateNode
	add := func(exited []*ast.StateNode) {
		for _, state := range exited {
			if !slices.Contains(states, state) {
				states = append(states, state)
			}
		}
	}
	for _, target := range targets {
		add(exits(target))
	}
	for _, stop := range stops {
		add(e.terminateExits(from, trans, stop))
	}
	return states, true
}
