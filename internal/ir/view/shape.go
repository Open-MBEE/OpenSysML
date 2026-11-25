package view

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Shape is the notation outline of a node, derived from its metaclass and
// rendering kind, independent of any written form.
type Shape string

const (
	// ShapeRectangle is a definition's square-cornered box.
	ShapeRectangle Shape = "rectangle"
	// ShapeRounded is a usage's rounded box.
	ShapeRounded Shape = "rounded"
	// ShapeStadium is the case family's stadium.
	ShapeStadium Shape = "stadium"
	// ShapeActor is the actor's stick figure.
	ShapeActor Shape = "actor"
	// ShapeNote is the folded-corner note of an annotation or an objective.
	ShapeNote Shape = "note"
	// ShapeDiamond is a decision, merge or choice node.
	ShapeDiamond Shape = "diamond"
	// ShapeCircle is a filled circle: an initial node, a junction or a n-ary dot.
	ShapeCircle Shape = "circle"
	// ShapeRing is the final node's hollow circle.
	ShapeRing Shape = "ring"
	// ShapeCross is the terminate node's crossed circle.
	ShapeCross Shape = "cross"
	// ShapeBar is the fork or join bar.
	ShapeBar Shape = "bar"
	// ShapeHistory is the history node's lettered circle.
	ShapeHistory Shape = "history"
	// ShapeItem is an item box drawn on a connection line, with no outline.
	ShapeItem Shape = "item"
)

// shapes lists every shape in the order the API documents them.
var shapes = []Shape{
	ShapeRectangle, ShapeRounded, ShapeStadium, ShapeActor, ShapeNote, ShapeDiamond,
	ShapeCircle, ShapeRing, ShapeCross, ShapeBar, ShapeHistory, ShapeItem,
}

// Shapes lists every notation shape in documentation order.
func Shapes() []Shape { return append([]Shape(nil), shapes...) }

// Valid reports whether s names a notation shape.
func (s Shape) Valid() bool {
	for _, shape := range shapes {
		if s == shape {
			return true
		}
	}
	return false
}

// NodeShape is the notation shape of node in a rendering of kind, following
// the cases the written forms draw without calling a writer.
func NodeShape(kind Kind, node NodeData) Shape {
	if node.Arrow != "" {
		return ShapeItem
	}
	caseRendering := kind == KindCase || kind == KindMixed
	if caseRendering && caseNodeKind(node.Kind) {
		return ShapeStadium
	}
	if caseRendering {
		switch node.Kind {
		case "actor":
			return ShapeActor
		case "subject":
			return ShapeRectangle
		case "objective":
			return ShapeNote
		}
	}
	switch node.Kind {
	case commentKind, docKind, repKind:
		return ShapeNote
	case startKind, "initial", "junction", connectionDotKind, connectionDefDotKind, dependencyDotKind:
		return ShapeCircle
	case "final":
		return ShapeRing
	case terminateKind:
		return ShapeCross
	case "fork", "join":
		return ShapeBar
	case "decision", "merge", "choice":
		return ShapeDiamond
	case shallowHistoryKind, deepHistoryKind:
		return ShapeHistory
	}
	keyword := ast.NotationKeyword(node.Kind)
	if isDefinitionKind(keyword) || slices.Contains(strings.Fields(node.Kind), "package") || keyword == "region" || keyword == swimlaneKind {
		return ShapeRectangle
	}
	return ShapeRounded
}
