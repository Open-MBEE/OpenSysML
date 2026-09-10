package ast

// ImplicitTransitionSource returns the member a sourceless transition leaves: the closest
// before it, past other sourceless transitions (SysML v2 §7.18.3); nil when none precedes it.
func ImplicitTransitionSource(members []Node, transition Node) Node {
	at := -1
	for i, member := range members {
		if member == transition || unwrapMember(member) == transition {
			at = i
			break
		}
	}
	for i := at - 1; i >= 0; i-- {
		if member := unwrapMember(members[i]); !transparentToImplicitSource(member) {
			return member
		}
	}
	return nil
}

// transparentToImplicitSource reports whether the previous-member rule looks past member:
// a sourceless transition, a succession (`then state s;` lists it after `s`) or a syntax error.
func transparentToImplicitSource(member Node) bool {
	switch n := member.(type) {
	case *TransitionMember:
		return n.Source == nil
	case *SuccessionEdge, *ControlFlowEdge, *ErrorNode:
		return true
	}
	return false
}
