package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
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
	return r.target != nil || r.choice != nil || r.draw != nil
}

// routeEffect is one effect of a compound transition and the state declaring the
// pseudostate its segment leaves; nil for a segment out of a state or the machine's body.
type routeEffect struct {
	behavior lower.StateBehavior
	within   *ast.StateNode
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
			effects = append(effects, routeEffect{behavior: behavior, within: within})
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
	default:
		return route{}, fmt.Errorf("transition target must be a state or pseudostate, got %T", trans.Target)
	}
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
	r.target, r.choice, r.draw = beyond.target, beyond.choice, beyond.draw
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
	default:
		return r, fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", from.Kind, from.Name, seg.Target)
	}
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
	e.ctx.noteAll(notes)
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
	e.ctx.noteAll(r.notes)
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
	point, ok := e.branchPoint(ps, outgoing, enabled)
	if !ok {
		return 0, nil
	}
	pick := e.ctx.scheduling().choose(point, nil)
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

// reachable lists the states the route open at a choice or a draw can end in:
// along the route settled beyond each branch of the junction enabled (one that
// could not be settled ends nowhere), or through every branch of the choice and
// whatever pseudostates lie beyond, each once.
func (e *StateExecutor) reachable(r route) ([]*ast.StateNode, error) {
	var states []*ast.StateNode
	seen := make(map[*ast.PseudostateNode]bool)
	for _, ps := range r.crossed {
		seen[ps] = true
	}
	add := func(target *ast.StateNode) {
		if !slices.Contains(states, target) {
			states = append(states, target)
		}
	}
	var visit func(ps *ast.PseudostateNode, branches []*lower.Transition) error
	visit = func(ps *ast.PseudostateNode, branches []*lower.Transition) error {
		for _, branch := range branches {
			switch target := branch.Target.(type) {
			case *ast.StateNode:
				add(target)
			case *ast.PseudostateNode:
				if !transientPseudostate(target.Kind) {
					return fmt.Errorf("%s %s: a transition into %s %s is not supported", ps.Kind, ps.Name, target.Kind, target.Name)
				}
				if seen[target] {
					continue
				}
				seen[target] = true
				if err := visit(target, e.graph.Transitions[target]); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", ps.Kind, ps.Name, branch.Target)
			}
		}
		return nil
	}
	var settled func(r route) error
	settled = func(r route) error {
		if r.target != nil {
			add(r.target)
			return nil
		}
		if r.draw != nil {
			for _, beyond := range r.draw.beyond {
				if beyond.err != nil {
					continue
				}
				if err := settled(beyond.route); err != nil {
					return err
				}
			}
			return nil
		}
		return visit(r.choice, e.graph.Transitions[r.choice])
	}
	return states, settled(r)
}

// exitPlan lists the states a move to target exits, against the configuration
// as it stands.
type exitPlan func(target *ast.StateNode) []*ast.StateNode

// entryPlan lists the states a move to target enters, outermost first, against
// the configuration as it stands.
type entryPlan func(target *ast.StateNode) []*ast.StateNode

// certainExits lists the states a move to every one of the targets exits — in a
// sibling region as well as above the source — in the order the first target's
// move leaves them, so they can be left before the branch is known.
func (e *StateExecutor) certainExits(targets []*ast.StateNode, exits exitPlan) []*ast.StateNode {
	if len(targets) == 0 {
		return nil
	}
	first := e.expandExits(exits(targets[0]))
	common := make(map[*ast.StateNode]bool, len(first))
	for _, state := range first {
		common[state] = true
	}
	for _, target := range targets[1:] {
		left := make(map[*ast.StateNode]bool)
		for _, state := range e.expandExits(exits(target)) {
			left[state] = true
		}
		for state := range common {
			if !left[state] {
				delete(common, state)
			}
		}
	}
	certain := make([]*ast.StateNode, 0, len(common))
	for _, state := range first {
		if common[state] {
			certain = append(certain, state)
		}
	}
	return certain
}

// certainEntries lists the states a move to every one of the targets enters, in
// the order the first target's move enters them, before the branch is known.
func (e *StateExecutor) certainEntries(targets []*ast.StateNode, enters entryPlan) []*ast.StateNode {
	if len(targets) == 0 {
		return nil
	}
	certain := enters(targets[0])
	for _, target := range targets[1:] {
		entered := enters(target)
		certain = slices.DeleteFunc(certain, func(state *ast.StateNode) bool {
			return !slices.Contains(entered, state)
		})
	}
	return certain
}

// runEffects performs a compound transition's effects in path order, activating the
// chain down to the state enclosing each first: a segment is a performance of its owner.
func (e *StateExecutor) runEffects(effects []routeEffect, chain []*ast.StateNode) error {
	for _, effect := range effects {
		if upto := e.enclosingIndex(chain, effect.within); upto >= 0 {
			if err := e.enterAhead(chain[:upto+1]); err != nil {
				return err
			}
		}
		if err := e.executeBehavior(effect.behavior); err != nil {
			return fmt.Errorf("transition effect: %w", err)
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

// travel takes a compound transition along r: a draw it is open at is made first,
// then at each choice it leaves the states every branch leaves, runs the effects
// of the segments into it and reads its guards; move then finishes the settled
// rest with the effects left.
func (e *StateExecutor) travel(r route, exits exitPlan, enters entryPlan, move func([]routeEffect, *ast.StateNode) error) error {
	return e.travelChoosing(r.choice != nil || r.draw != nil, r, exits, enters, move)
}

// travelChoosing is travel where choosing says whether a draw lies on the way, on
// the route or inside move, at which a replay may refuse the move.
func (e *StateExecutor) travelChoosing(choosing bool, r route, exits exitPlan, enters entryPlan, move func([]routeEffect, *ast.StateNode) error) error {
	savedLeft, savedEntered := e.leftAhead, e.enteredAhead
	e.leftAhead, e.enteredAhead = nil, nil
	defer func() { e.leftAhead, e.enteredAhead = savedLeft, savedEntered }()
	var err error
	if !choosing {
		err = e.travelResolving(r, exits, enters, move)
	} else {
		err = e.moveWhole(func() error { return e.travelResolving(r, exits, enters, move) })
	}
	if err != nil {
		return err
	}
	return e.entriesSettled()
}

// moveWhole makes move as one compound transition. Only a replay refuses a move,
// at a draw the draws, exits and effects ahead of it have been made for; a
// refused move is undone whole.
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
// effects into it are done.
func (e *StateExecutor) travelResolving(r route, exits exitPlan, enters entryPlan, move func([]routeEffect, *ast.StateNode) error) error {
	if r.draw != nil {
		var err error
		r, err = e.settleDraws(r)
		e.ctx.noteAll(r.notes)
		r.notes = nil
		if err != nil {
			return err
		}
	}
	for r.choice != nil {
		targets, err := e.reachable(r)
		if err != nil {
			return err
		}
		if err := e.exitAhead(e.certainExits(targets, exits)); err != nil {
			return err
		}
		if err := e.runEffects(r.effects(e.graph), e.certainEntries(targets, enters)); err != nil {
			return err
		}
		if err := e.enterOwnerOf(r.choice, e.certainEntries(targets, enters)); err != nil {
			return err
		}
		e.noteFired(r.segments...)
		if r, err = e.resolveChoice(r); err != nil {
			return err
		}
	}
	e.noteFired(r.segments...)
	return move(r.effects(e.graph), r.target)
}

// runBehaviors performs a transition's effects, in order.
func (e *StateExecutor) runBehaviors(effects []lower.StateBehavior) error {
	for _, behavior := range effects {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	return nil
}

// mayExit lists the states a compound transition along r may leave, whichever
// state it can end at; false where the route cannot be followed.
func (e *StateExecutor) mayExit(r route, exits exitPlan) ([]*ast.StateNode, bool) {
	if r.target != nil {
		return exits(r.target), true
	}
	targets, err := e.reachable(r)
	if err != nil {
		return nil, false
	}
	var states []*ast.StateNode
	for _, target := range targets {
		for _, state := range exits(target) {
			if !slices.Contains(states, state) {
				states = append(states, state)
			}
		}
	}
	return states, true
}
