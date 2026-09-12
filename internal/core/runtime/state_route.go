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
// run, the transition first, ending at a state or open at a choice whose guards
// are read only once those segments' effects have run. Junctions are settled on
// the way; neither is set for a fork, a waiting join or a history restoring.
type route struct {
	segments []*lower.Transition
	target   *ast.StateNode
	choice   *ast.PseudostateNode
	// crossed are the pseudostates passed, so a path back into one is a cycle.
	crossed []*ast.PseudostateNode
}

// settled reports whether the route has somewhere to move to.
func (r route) settled() bool {
	return r.target != nil || r.choice != nil
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
// on through junctions, a synchronized join or an unrecorded history's default
// transition, up to the first choice.
func (e *StateExecutor) resolveRoute(trans *lower.Transition) (route, error) {
	r := route{segments: []*lower.Transition{trans}}
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		r.target = target
		return r, nil
	case *ast.PseudostateNode:
		switch target.Kind {
		case ast.PseudostateFork:
			return r, nil
		case ast.PseudostateJoin:
			sources, err := e.joinSources(target)
			if err != nil {
				return route{}, err
			}
			if !e.allActive(sources) {
				return r, nil
			}
		case ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
			owner := e.graph.PseudostateOwner[target]
			if owner == nil || e.historyRecorded(owner) || len(e.graph.Transitions[target]) == 0 {
				return r, nil
			}
			r, err := e.followOut(target, r)
			if err != nil {
				return route{}, fmt.Errorf("default transition of history %s, %s having no recorded configuration: %w", target.Name, owner.Name, err)
			}
			return r, nil
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
// route open there; out of any other the first segment whose guard holds is taken.
func (e *StateExecutor) followOut(ps *ast.PseudostateNode, r route) (route, error) {
	if slices.Contains(r.crossed, ps) {
		return route{}, fmt.Errorf("%s %s: outgoing transitions form a cycle between pseudostates", ps.Kind, ps.Name)
	}
	r.crossed = append(r.crossed, ps)
	if ps.Kind == ast.PseudostateChoice {
		r.choice = ps
		return r, nil
	}
	branch, err := e.pseudostateBranch(ps)
	if err != nil {
		return route{}, err
	}
	return e.follow(ps, branch, r)
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

// pseudostateBranch returns the outgoing transition a junction, join or history
// routes along: the first whose guard is satisfied, in declaration order, an
// unguarded one being the default branch. Exactly one succession is taken, as
// KerML `DecisionPerformance::outgoingHBLink: HappensBefore[1]` requires.
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

// resolveChoice reads the guards of the choice the route is open at against the
// data as it now stands and goes on from the branch taken: the policy draws among
// several enabled, as a recorded choice point; an unguarded branch is the else
// branch, taken when no guard holds; none enabled is the typed error.
func (e *StateExecutor) resolveChoice(r route) (route, error) {
	choice := r.choice
	outgoing := e.graph.Transitions[choice]
	var enabled, unguarded []int
	var notes []UnevaluableGuard
	for i, trans := range outgoing {
		if trans.Guard == nil {
			unguarded = append(unguarded, i)
			continue
		}
		// Once a branch holds the rest are probed only to report the choice; one
		// with no result is not a branch, and is noted.
		var pass bool
		if len(enabled) > 0 {
			var unevaluable *UnevaluableGuard
			if pass, unevaluable = e.probeBranch(choice, outgoing, i); unevaluable != nil {
				notes = append(notes, *unevaluable)
			}
		} else {
			var err error
			if pass, err = e.passesGuard(trans); err != nil {
				return route{}, fmt.Errorf("choice %s: %w", choice.Name, err)
			}
		}
		if pass {
			enabled = append(enabled, i)
		}
	}
	for _, note := range notes {
		e.ctx.noteUnevaluableGuard(note)
	}
	if len(enabled) == 0 {
		enabled = unguarded
	}
	if len(enabled) == 0 {
		return route{}, fmt.Errorf("%w: choice %s: no guard evaluated to true", ErrChoiceWithoutBranch, choice.Name)
	}
	pick := 0
	if point, ok := e.choiceBranchPoint(choice, outgoing, enabled); ok {
		pick = e.ctx.scheduling().choose(point, nil)
		// A refused replay move takes no branch; travel undoes the move made so far.
		if err := e.ctx.scheduling().refusal(); err != nil {
			return route{}, err
		}
		point.Taken = pick
		point.File, point.Span = e.transitionLocation(choice, outgoing[enabled[pick]])
		e.ctx.note(point)
		// A branch past the first was only probed; its guard's final reading is made now.
		if pick > 0 {
			if _, err := e.passesGuard(outgoing[enabled[pick]]); err != nil {
				return route{}, fmt.Errorf("choice %s: %w", choice.Name, err)
			}
		}
	}
	return e.follow(choice, outgoing[enabled[pick]], route{crossed: r.crossed})
}

// probeBranch reads whether the branch at position i out of choice holds once
// another already does, as a probe the context undoes whole; one that cannot be
// evaluated is not enabled and is returned as the note to record.
func (e *StateExecutor) probeBranch(choice *ast.PseudostateNode, outgoing []*lower.Transition, i int) (bool, *UnevaluableGuard) {
	var pass bool
	var err error
	e.preview(func() { pass, err = e.passesGuard(outgoing[i]) })
	if err != nil {
		file, _ := e.transitionLocation(choice, outgoing[i])
		return false, &UnevaluableGuard{
			Where:       "choice " + choice.Name,
			Alternative: transitionName(outgoing, i),
			Reason:      err.Error(),
			File:        file,
			Span:        outgoing[i].Guard.Span(),
		}
	}
	return pass, nil
}

// choiceBranchPoint is the branches of a choice enabled on arrival, at their
// declared positions, as the choice point still to be taken; there is none
// under two.
func (e *StateExecutor) choiceBranchPoint(choice *ast.PseudostateNode, outgoing []*lower.Transition, enabled []int) (ChoicePoint, bool) {
	if len(enabled) < 2 {
		return ChoicePoint{}, false
	}
	alts := make([]string, len(enabled))
	for i, pos := range enabled {
		alts[i] = transitionName(outgoing, pos)
	}
	return ChoicePoint{
		Kind:         ChoiceTransition,
		Where:        "choice " + choice.Name,
		Alternatives: alts,
	}, true
}

// reachable lists the states the branches of the choice the route is open at can
// end in, through whatever pseudostates lie beyond it, each once.
func (e *StateExecutor) reachable(r route) ([]*ast.StateNode, error) {
	var states []*ast.StateNode
	seen := make(map[*ast.PseudostateNode]bool)
	for _, ps := range r.crossed {
		seen[ps] = true
	}
	var visit func(ps *ast.PseudostateNode) error
	visit = func(ps *ast.PseudostateNode) error {
		for _, branch := range e.graph.Transitions[ps] {
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
				if err := visit(target); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", ps.Kind, ps.Name, branch.Target)
			}
		}
		return nil
	}
	return states, visit(r.choice)
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
	saved := e.leftAhead
	e.leftAhead = nil
	defer func() { e.leftAhead = saved }()
	if r.choice == nil || !e.ctx.scheduling().replaying() {
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
	if r.choice == nil {
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
