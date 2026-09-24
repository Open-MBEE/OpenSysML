package view

// Direction is the flow direction a graph-shaped rendering is drawn in, as
// Mermaid names them.
type Direction string

const (
	DirectionTopBottom Direction = "TB"
	DirectionLeftRight Direction = "LR"
	DirectionRightLeft Direction = "RL"
	DirectionBottomTop Direction = "BT"
)

// ParseDirection reports the direction name names, and whether it names one.
func ParseDirection(name string) (Direction, bool) {
	switch Direction(name) {
	case DirectionTopBottom, DirectionLeftRight, DirectionRightLeft, DirectionBottomTop:
		return Direction(name), true
	}
	return "", false
}

// SupportsDirection reports whether a rendering of the kind is drawn as a
// graph a direction can be stated for: a flowchart or a state diagram. A
// table has no direction, and a sequence diagram always reads top-down.
func (k Kind) SupportsDirection() bool {
	switch k {
	case KindTree, KindInterconnection, KindState, KindAction:
		return true
	}
	return false
}
