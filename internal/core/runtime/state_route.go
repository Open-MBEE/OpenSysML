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
// their guards read when the transition was selected; the policy draws among them
// only once the transition is committed to fire, so a transition another region's
// reaction leaves behind draws nothing.
type junctionDraw struct {
	at       *ast.PseudostateNode
	outgoing []*lower.Transition
	enabled  []int
}

// settled reports whether the route has somewhere to move to.
func (r route) settled() bool {
	return r.target != nil || r.choice != nil || r.draw != nil
}

// effects are the behaviors the route's segments perform, in path order.
func (r route) effects() []lower.StateBehavior {
	var effects []lower.StateBehavior
	for _, seg := range r.segments {
		effects = append(effects, seg.Effect...)
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
func (e *StateExecutor) followOut(ps *ast.PseudostateNode, r route) (route, error) {
	if slices.Contains(r.crossed, ps) {
		return route{}, fmt.Errorf("%s %s: outgoing transitions form a cycle between pseudostates", ps.Kind, ps.Name)
	}
	r.crossed = append(r.crossed, ps)
	if ps.Kind == ast.PseudostateChoice {
		r.choice = ps
		return r, nil
	}
	outgoing := e.graph.Transitions[ps]
	if len(outgoing) == 0 {
		return route{}, fmt.Errorf("%s %s has no outgoing transitions", ps.Kind, ps.Name)
	}
	enabled, notes, err := e.enabledBranches(ps, outgoing)
	if err != nil {
		return route{}, err
	}
	r.notes = append(r.notes, notes...)
	if len(enabled) == 0 {
		return route{}, fmt.Errorf("%s %s: no guard evaluated to true", ps.Kind, ps.Name)
	}
	if len(enabled) > 1 {
		r.draw = &junctionDraw{at: ps, outgoing: outgoing, enabled: enabled}
		return r, nil
	}
	return e.follow(ps, outgoing[enabled[0]], r)
}

// settleDraws makes the draws the route is open at, in turn, once the transition
// is committed to fire and nothing has moved yet: the policy draws among the
// enabled branches, as a choice point among the route's notes, and the route goes
// on from the branch drawn, through whatever lies beyond; a draw the witness
// refuses is the refusal, and the route is left where it was.
func (e *StateExecutor) settleDraws(r route) (route, error) {
	for r.draw != nil {
		draw := r.draw
		r.draw = nil
		pick, err := e.pickBranch(draw.at, draw.outgoing, draw.enabled, func(n RunNote) { r.notes = append(r.notes, n) })
		if err != nil {
			return route{}, err
		}
		if r, err = e.follow(draw.at, draw.outgoing[draw.enabled[pick]], r); err != nil {
			return route{}, fmt.Errorf("evaluate pseudostate: %w", err)
		}
	}
	return r, nil
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
			return route{}, fmt.Errorf("%s %s: a transition into %s %s is not supported", from.Kind, from.Name, target.Kind, target.Name)
		}
		return e.followOut(target, r)
	default:
		return route{}, fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", from.Kind, from.Name, seg.Target)
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
// point taken; a draw the witness refuses is the refusal.
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
	// A branch past the first was only probed; its guard's final reading is made now.
	if pick > 0 {
		if _, err := e.passesGuard(outgoing[enabled[pick]]); err != nil {
			return 0, fmt.Errorf("%s %s: %w", ps.Kind, ps.Name, err)
		}
	}
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
// through every branch of the choice, or the branches of the junction enabled,
// and whatever pseudostates lie beyond, each once.
func (e *StateExecutor) reachable(r route) ([]*ast.StateNode, error) {
	var states []*ast.StateNode
	seen := make(map[*ast.PseudostateNode]bool)
	for _, ps := range r.crossed {
		seen[ps] = true
	}
	var visit func(ps *ast.PseudostateNode, branches []*lower.Transition) error
	visit = func(ps *ast.PseudostateNode, branches []*lower.Transition) error {
		for _, branch := range branches {
			switch target := branch.Target.(type) {
			case *ast.StateNode:
				if !slices.Contains(states, target) {
					states = append(states, target)
				}
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
	if r.draw != nil {
		branches := make([]*lower.Transition, len(r.draw.enabled))
		for i, pos := range r.draw.enabled {
			branches[i] = r.draw.outgoing[pos]
		}
		return states, visit(r.draw.at, branches)
	}
	return states, visit(r.choice, e.graph.Transitions[r.choice])
}

// exitPlan lists the states a move to target exits, against the configuration
// as it stands.
type exitPlan func(target *ast.StateNode) []*ast.StateNode

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

// travel takes a compound transition along r: at each choice it leaves the
// states every branch leaves, runs the effects of the segments into it and
// reads its guards; move then finishes the settled rest with the effects left.
func (e *StateExecutor) travel(r route, exits exitPlan, move func([]lower.StateBehavior, *ast.StateNode) error) error {
	return e.travelChoosing(r.choice != nil, r, exits, move)
}

// travelChoosing is travel where choosing says whether a choice lies on the way,
// on the route or inside move, at which a replay may refuse the move.
func (e *StateExecutor) travelChoosing(choosing bool, r route, exits exitPlan, move func([]lower.StateBehavior, *ast.StateNode) error) error {
	saved := e.leftAhead
	e.leftAhead = nil
	defer func() { e.leftAhead = saved }()
	if !choosing || !e.ctx.scheduling().replaying() {
		return e.travelResolving(r, exits, move)
	}
	// Only a replay refuses a move, at a choice the exits and effects ahead of it
	// have been made for; a refused move is undone whole.
	mark := e.markMove()
	err := e.travelResolving(r, exits, move)
	if e.ctx.scheduling().refusal() != nil {
		mark.undo()
	} else {
		mark.keep()
	}
	return err
}

// travelResolving is travel's course: each choice on the way resolved once the
// exits every branch makes and the effects into it are done.
func (e *StateExecutor) travelResolving(r route, exits exitPlan, move func([]lower.StateBehavior, *ast.StateNode) error) error {
	for r.choice != nil {
		targets, err := e.reachable(r)
		if err != nil {
			return err
		}
		if err := e.exitAhead(e.certainExits(targets, exits)); err != nil {
			return err
		}
		if err := e.runBehaviors(r.effects()); err != nil {
			return err
		}
		if r, err = e.resolveChoice(r); err != nil {
			return err
		}
	}
	return move(r.effects(), r.target)
}

// runBehaviors performs the effects of a compound transition's segments, in order.
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
