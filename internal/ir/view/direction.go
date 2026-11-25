package view

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

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
	if slices.Contains(Directions(), Direction(name)) {
		return Direction(name), true
	}
	return "", false
}

// Directions are the directions a rendering can be asked for.
func Directions() []Direction {
	return []Direction{DirectionTopBottom, DirectionLeftRight, DirectionRightLeft, DirectionBottomTop}
}

// DirectionNames spells the directions as a list, for help and error text.
func DirectionNames() string {
	names := make([]string, 0, len(Directions()))
	for _, direction := range Directions() {
		names = append(names, string(direction))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownDirection is a direction name that names none. UnknownDirectionError wraps it.
var ErrUnknownDirection = errors.New("unknown direction")

// UnknownDirectionError is a direction asked for by a name none has; it names
// the directions there are.
type UnknownDirectionError struct {
	Name string
}

func (e *UnknownDirectionError) Error() string {
	return fmt.Sprintf("unknown direction %q; the directions are %s", e.Name, DirectionNames())
}

func (e *UnknownDirectionError) Unwrap() error { return ErrUnknownDirection }

// SupportsDirection reports whether a rendering of the kind is drawn as a
// graph a direction can be stated for: a flowchart or a state diagram. A
// table has no direction, and a sequence diagram always reads top-down.
func (k Kind) SupportsDirection() bool {
	switch k {
	case KindTree, KindInterconnection, KindState, KindAction, KindCase, KindMixed, KindRequirement, KindDefinition, KindPackage:
		return true
	}
	return false
}
