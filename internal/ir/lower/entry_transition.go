package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// EntryTransition is a transition out of a body's entry action. Alternatives
// are kept in declaration order, the order the guards are tried in.
type EntryTransition struct {
	// Decl is the member as written: a TransitionMember, a SuccessionEdge, a
	// succession usage or a `first start then s;` marker.
	Decl ast.Node
	// Guard is nil for an unconditional entry transition.
	Guard  ast.Node
	Target *ast.StateNode
	Via    *ast.PseudostateNode
	Effect []StateBehavior
	// Scope is the scope Guard resolves in.
	Scope *symbols.Scope
}

const (
	EntryTransitionAccepterSourceMessage = "A transition with an accepter must have a state as its source. " +
		"(SysML v2; pilot validateTransitionUsageTriggerActions)"
	EntryTransitionShorthandEffectMessage = "a shorthand transition out of an entry action may carry a guard at most; " +
		"write `entry action boot; transition boot do action mark then s;` to give the transition an explicit source " +
		"(SysML v2 7.18.3 EntryTransitionMember)"
)

// EntryTransitionTargetFormat reports a transition out of the entry action whose
// target is a vertex the body cannot start in.
const EntryTransitionTargetFormat = "transition out of the entry action reaches %s, " +
	"which is not a state or junction/choice route the body can start in (SysML v2 7.18.3)"

// EntryTransitionShapeError marks a triggered or shorthand effect transition
// out of the entry action.
type EntryTransitionShapeError struct {
	Transition *ast.TransitionMember
}

func (e *EntryTransitionShapeError) Error() string {
	if e.Transition.Trigger != nil {
		return EntryTransitionAccepterSourceMessage
	}
	return EntryTransitionShorthandEffectMessage
}

// EntryTransitionTargetError marks a transition out of the entry action whose
// target is not a state or an allowed junction/choice.
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
	return g.EntryTransitions[g.EntryOwner(owner)]
}

// EntryOwner is the body owner's entry transitions are written in, as
// EntryTransitions keys it: the region a state of a parallel state stands for,
// else owner itself.
func (g *StateGraph) EntryOwner(owner ast.Node) ast.Node {
	if state, ok := owner.(*ast.StateNode); ok {
		return g.entryOwner(state)
	}
	return owner
}

// UnconditionalStart is the state a body starts in whatever its guards say.
func (g *StateGraph) UnconditionalStart(owner ast.Node) *ast.StateNode {
	transitions := g.StartOf(owner)
	if len(transitions) == 0 || transitions[0].Guard != nil {
		return nil
	}
	switch target := entryTransitionTarget(transitions[0]).(type) {
	case *ast.StateNode:
		return target
	case *ast.PseudostateNode:
		return g.unconditionalEntryTarget(target, nil)
	default:
		return nil
	}
}

func (g *StateGraph) unconditionalEntryTarget(target ast.Node, seen map[*ast.PseudostateNode]bool) *ast.StateNode {
	switch node := target.(type) {
	case *ast.StateNode:
		return node
	case *ast.PseudostateNode:
		if seen[node] || len(g.Transitions[node]) != 1 || g.Transitions[node][0].Guard != nil {
			return nil
		}
		if seen == nil {
			seen = make(map[*ast.PseudostateNode]bool)
		}
		seen[node] = true
		return g.unconditionalEntryTarget(g.Transitions[node][0].Target, seen)
	default:
		return nil
	}
}

// addEntryTransition records a transition out of a body's entry action.
func (g *StateGraph) addEntryTransition(owner ast.Node, transition *EntryTransition) {
	g.EntryTransitions[owner] = append(g.EntryTransitions[owner], transition)
	g.recordDeclaredIn(transition.Decl, transition.Scope)
	g.designateInitial(entryTransitionTarget(transition))
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
			g.designateInitial(entryTransitionTarget(transition))
		}
	}
}

func entryTransitionTarget(transition *EntryTransition) ast.Node {
	if transition.Via != nil {
		return transition.Via
	}
	return transition.Target
}

// entryOwner is the body an entry transition written in state's body starts: the
// region when state stands for one of a parallel state, else the state itself.
func (g *StateGraph) entryOwner(state *ast.StateNode) ast.Node {
	if region := g.HiddenRegionOf[state]; region != nil {
		return region
	}
	return state
}

// lowerEntryTransition lowers a sourceless transition whose preceding member is
// the entry action into an entry transition of entryOwner's body.
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
	entryScope := symbols.TriggerScope(scope, member)
	return addEntryTransitionTarget(graph, member, target, entryOwner, member.Guard, nil, scope, entryScope)
}

func addEntryTransitionTarget(
	graph *StateGraph,
	decl ast.Node,
	target ast.Node,
	owner ast.Node,
	guard ast.Node,
	effect []StateBehavior,
	targetScope *symbols.Scope,
	scope *symbols.Scope,
) error {
	entry := &EntryTransition{Decl: decl, Guard: guard, Effect: effect, Scope: scope}
	switch vertex := target.(type) {
	case *ast.StateNode:
		entry.Target = vertex
	case *ast.PseudostateNode:
		if vertex.Kind != ast.PseudostateJunction && vertex.Kind != ast.PseudostateChoice ||
			graph.declaredIn[vertex] != targetScope {
			return &EntryTransitionTargetError{Target: target}
		}
		entry.Via = vertex
	default:
		return &EntryTransitionTargetError{Target: target}
	}
	graph.addEntryTransition(owner, entry)
	return nil
}
