package ast

// IsSuccessionSource reports whether a positional `then` sequences from this body
// member: a feature that is not an edge (SysML v2 §7.17.4). Parser and RDF writer share it.
func IsSuccessionSource(member Node) bool {
	switch n := member.(type) {
	case *Membership:
		return n.Member != nil && IsSuccessionSource(n.Member)
	case *Usage:
		// A directioned usage is a parameter, not a node a flow can leave.
		return n.Direction == DirNone && !n.Kind.IsEdge()
	case *SuccessionEdge, *ControlFlowEdge, *ObjectFlowEdge, *TransitionEdge, *TransitionMember:
		return false
	case *Definition, *Package, *Namespace, *Import, *Alias, *Dependency, *MultiplicityDecl,
		*RelationshipMember, *Comment, *Documentation, *TextualRepresentation, *FilterMember:
		return false
	}
	return true
}

// SuccessionAdmitsBody reports whether the `then` edge among owner's members may end in a UsageBody: an ActionTargetSuccession
// (SysML.xtext:1698, `then done` included) or a state body's TargetTransitionUsage (:1764) does; a member-attached or entry `then` not.
func SuccessionAdmitsBody(owner, edge Node) bool {
	if e, ok := edge.(*SuccessionEdge); ok && (e.TargetImplied || e.TargetMember != nil) && !targetsDone(e) {
		return false
	}
	switch d := owner.(type) {
	case *SuccessionEdge:
		return true
	case *Definition:
		switch d.Kind {
		case DefAction, DefBehavior:
			return true
		case DefState:
			return IsStateTargetTransition(owner, edge)
		}
	case *Usage:
		switch d.Kind {
		case UsageAction, UsageTransition, UsageStep, UsageBehavior:
			return true
		case UsageState:
			return IsStateTargetTransition(owner, edge)
		}
	}
	return false
}

// targetsDone reports whether a `then done` edge reaches the action's `done` node, a TargetSuccession
// whose target is the library's end node rather than a member the keyword introduces.
func targetsDone(e *SuccessionEdge) bool {
	_, ok := e.TargetMember.(*FinalNode)
	return ok
}

// IsStateTargetTransition reports whether edge, a `then x;` among the members of the state owner, is a TargetTransitionUsage
// (SysML.xtext:1764, :1866): written after a state, not after `entry`/`do`/`exit` (:1796) nor member-attached (:1046).
func IsStateTargetTransition(owner, edge Node) bool {
	e, ok := edge.(*SuccessionEdge)
	if !ok || e.TargetImplied || e.TargetMember != nil {
		return false
	}
	switch d := owner.(type) {
	case *Definition:
		return d.Kind == DefState && thenBodyAllowedAt(d.Members, edge)
	case *Usage:
		return d.Kind == UsageState && thenBodyAllowedAt(d.Members, edge)
	}
	return false
}

// thenBodyAllowedAt walks a state body's members to edge, as the parser did reading them.
func thenBodyAllowedAt(members []Node, edge Node) bool {
	allowed := true
	for _, member := range members {
		if member == edge {
			return allowed
		}
		allowed = ThenBodyAllowedAfter(member, allowed)
	}
	return false
}

// ThenBodyAllowedAfter reports whether a `then` after member in a state body may carry a body:
// only a TargetTransitionUsage does (SysML.xtext:1764), not an EntryTransitionMember (:1796-1801).
func ThenBodyAllowedAfter(member Node, allowed bool) bool {
	switch member.(type) {
	case *EntryMember, *DoMember, *ExitMember:
		return false
	case *SuccessionEdge, *ControlFlowEdge:
		return allowed
	}
	return true
}
