package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EntryTransition is a transition out of a body's entry action, naming a state the
// body starts in: `entry; then s;` always, `entry; if c then s;` when c holds
// (SysML v2 7.18.3 EntryTransitionMember). A body's alternatives are kept in
// declaration order, the order the guards are tried in.
type EntryTransition struct {
	// Decl is the member as written: a TransitionMember, a SuccessionEdge, a
	// succession usage or a `first start then s;` marker.
	Decl ast.Node
	// Guard is nil for an unconditional entry transition.
	Guard  ast.Node
	Target *ast.StateNode
	// Scope is the scope Guard resolves in.
	Scope *symbols.Scope
}

// EntryTransitionShapeFormat reports a transition out of the entry action written
// with a trigger or an effect, which chooses a starting state by its guard alone.
const EntryTransitionShapeFormat = "transition out of the entry action carries %s: a transition out of the entry action " +
	"chooses the state its body starts in by its guard alone, `entry; if c then s;` or `entry; then s;` (SysML v2 7.18.3)"

// EntryTransitionTargetFormat reports a transition out of the entry action whose
// target is a vertex the body cannot start in.
const EntryTransitionTargetFormat = "transition out of the entry action reaches %s, " +
	"which is not a state to start in (SysML v2 7.18.3)"

// EntryTransitionShapeError marks a transition out of the entry action written
// with a trigger or an effect.
type EntryTransitionShapeError struct {
	Transition *ast.TransitionMember
}

func (e *EntryTransitionShapeError) Error() string {
	return fmt.Sprintf(EntryTransitionShapeFormat, EntryTransitionCarries(e.Transition))
}

// EntryTransitionCarries names what a transition out of the entry action was
// written with beyond its guard: "a trigger", "an effect" or both.
func EntryTransitionCarries(transition *ast.TransitionMember) string {
	switch {
	case transition.Trigger != nil && len(transition.Effect) > 0:
		return "a trigger and an effect"
	case transition.Trigger != nil:
		return "a trigger"
	}
	return "an effect"
}

// EntryTransitionTargetError marks a transition out of the entry action whose
// Target is a vertex but not a state.
type EntryTransitionTargetError struct {
	Target ast.Node
}

func (e *EntryTransitionTargetError) Error() string {
	return fmt.Sprintf(EntryTransitionTargetFormat, DescribeMember(e.Target))
}

// IsEntryTransition reports whether the member a sourceless transition leaves is
// the entry action of its body, whatever else the transition was written with.
func IsEntryTransition(source ast.Node) bool {
	_, entry := source.(*ast.EntryMember)
	return entry
}

// StartOf returns the transitions out of a body's entry action in declaration
// order: owner is the state whose body it is, the region, or nil for the
// machine's own body. A state standing for a region of a parallel state starts
// where that region's entry transitions say.
func (g *StateGraph) StartOf(owner ast.Node) []*EntryTransition {
	if state, ok := owner.(*ast.StateNode); ok {
		owner = g.entryOwner(state)
	}
	return g.EntryTransitions[owner]
}

// unconditionalStart returns the state a body starts in when no guard decides
// it: the target of its first entry transition when that one is unguarded.
func (g *StateGraph) unconditionalStart(owner ast.Node) *ast.StateNode {
	transitions := g.EntryTransitions[owner]
	if len(transitions) == 0 || transitions[0].Guard != nil {
		return nil
	}
	return transitions[0].Target
}

// addEntryTransition records a transition out of a body's entry action, which
// designates its target as a state the machine may start in.
func (g *StateGraph) addEntryTransition(owner ast.Node, transition *EntryTransition) {
	g.EntryTransitions[owner] = append(g.EntryTransitions[owner], transition)
	g.designateInitial(transition.Target)
}

// withOwnEntryTransitions runs collect over the members a body writes itself.
// Entry transitions written there replace those the body inherited, as its own
// entry behavior replaces the inherited one (pickBehaviors).
func (g *StateGraph) withOwnEntryTransitions(owner ast.Node, collect func() error) error {
	inherited := len(g.EntryTransitions[owner])
	if err := collect(); err != nil {
		return err
	}
	if transitions := g.EntryTransitions[owner]; inherited > 0 && len(transitions) > inherited {
		g.EntryTransitions[owner] = transitions[inherited:]
		g.redesignateInitials()
	}
	return nil
}

// redesignateInitials recomputes the states some entry transition names, once a
// body's inherited entry transitions have been dropped.
func (g *StateGraph) redesignateInitials() {
	g.designatedInitials = make(map[*ast.StateNode]bool)
	for _, transitions := range g.EntryTransitions {
		for _, transition := range transitions {
			g.designateInitial(transition.Target)
		}
	}
}

// entryOwner is the body an entry transition written in state's body starts: the
// region when state stands for one of a parallel state, else the state itself.
func (g *StateGraph) entryOwner(state *ast.StateNode) ast.Node {
	if region := g.HiddenRegionOf[state]; region != nil {
		return region
	}
	return state
}

// lowerEntryTransition lowers `entry; if c then s;` — a sourceless transition whose
// preceding member is the entry action — into an entry transition of entryOwner's
// body; owner is the body a `done` target completes.
func lowerEntryTransition(graph *StateGraph, member *ast.TransitionMember, owner, entryOwner ast.Node, scope *symbols.Scope) error {
	if member.Trigger != nil || len(member.Effect) > 0 {
		return &EntryTransitionShapeError{Transition: member}
	}
	if member.Target == nil {
		return fmt.Errorf("transition %s names no target", orAnonymous(member.Name))
	}
	target, err := graph.targetVertex(scope, member.Target, owner)
	if err != nil || target == nil {
		return err
	}
	state, ok := target.(*ast.StateNode)
	if !ok {
		return &EntryTransitionTargetError{Target: target}
	}
	graph.addEntryTransition(entryOwner, &EntryTransition{
		Decl:   member,
		Guard:  member.Guard,
		Target: state,
		Scope:  symbols.TriggerScope(scope, member),
	})
	return nil
}
