package program

import "strconv"

// NodeKind is what a node of an action graph does when a token reaches it.
type NodeKind uint8

const (
	// NodeNone is the zero kind, carried by no node.
	NodeNone NodeKind = iota
	// NodeInitial is the flow's entry, `first start`.
	NodeInitial
	// NodeFinal is a node the flow ends at, `done`.
	NodeFinal
	// NodeFork splits a token across every outgoing succession.
	NodeFork
	// NodeJoin waits for a token on every incoming succession.
	NodeJoin
	// NodeMerge passes a token on from any incoming succession.
	NodeMerge
	// NodeDecision chooses one guarded outgoing succession.
	NodeDecision
	// NodeExecution runs a nested action, `action a { ... }`.
	NodeExecution
	// NodePerform performs a referenced action, `perform action a`.
	NodePerform
	// NodeAccept waits for an event, `accept e`.
	NodeAccept
	// NodeSend dispatches an event, `send e to t`.
	NodeSend
	// NodeAssign writes a feature, `assign x := e`.
	NodeAssign
	// NodeTerminate ends the enclosing behavior, `terminate`.
	NodeTerminate
	// NodeIf branches on a condition, `if c { ... }`.
	NodeIf
	// NodeLoop repeats a body, `while c { ... }` and `for i in s { ... }`.
	NodeLoop
	// NodeBlock runs a body-local block of nodes.
	NodeBlock
	// NodeStatement runs the statements written between two nodes of a block as one step.
	NodeStatement
)

var nodeKindNames = [...]string{
	NodeNone:      "none",
	NodeInitial:   "initial",
	NodeFinal:     "final",
	NodeFork:      "fork",
	NodeJoin:      "join",
	NodeMerge:     "merge",
	NodeDecision:  "decision",
	NodeExecution: "action",
	NodePerform:   "perform",
	NodeAccept:    "accept",
	NodeSend:      "send",
	NodeAssign:    "assign",
	NodeTerminate: "terminate",
	NodeIf:        "if",
	NodeLoop:      "loop",
	NodeBlock:     "block",
	NodeStatement: "statement",
}

// String is the kind's word in the notation.
func (k NodeKind) String() string {
	if int(k) < len(nodeKindNames) {
		return nodeKindNames[k]
	}
	return "NodeKind(" + strconv.Itoa(int(k)) + ")"
}
