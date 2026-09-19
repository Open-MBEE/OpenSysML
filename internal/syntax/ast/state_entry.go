package ast

// EntryActions returns the actions the entry subactions among a state body's
// members declare, in declaration order.
func EntryActions(members []Node) []Node {
	var actions []Node
	for _, member := range members {
		if entry, ok := entryMember(member); ok {
			for _, action := range entry.Actions {
				actions = append(actions, unwrapMember(action))
			}
		}
	}
	return actions
}

// StateEntryActions returns the entry actions of a state declaration, whichever
// shape it was parsed into: a state node's entry behavior or a body's `entry`.
func StateEntryActions(state Node) []Node {
	switch n := state.(type) {
	case *StateNode:
		actions := make([]Node, 0, len(n.Entry))
		for _, action := range n.Entry {
			actions = append(actions, unwrapMember(action))
		}
		return actions
	case *Usage:
		return EntryActions(n.Members)
	case *Definition:
		return EntryActions(n.Members)
	}
	return nil
}

// IsEntryAction reports whether decl is one of the actions.
func IsEntryAction(actions []Node, decl Node) bool {
	for _, action := range actions {
		if action == decl {
			return true
		}
	}
	return false
}

// entryMember returns the entry subaction a body member declares, if it is one.
func entryMember(member Node) (*EntryMember, bool) {
	entry, ok := unwrapMember(member).(*EntryMember)
	return entry, ok
}

// unwrapMember returns what a membership owns, or the node itself.
func unwrapMember(node Node) Node {
	if membership, ok := node.(*Membership); ok {
		return membership.Member
	}
	return node
}
