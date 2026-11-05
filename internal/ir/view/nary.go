package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// The n-ary dot (SysML v2 §8.2.3.3, §8.2.3.13): a connection, connection definition or
// dependency of more than two ends is a filled dot carrying the name, one segment per end.
const (
	connectionDotKind    = "connection dot"
	connectionDefDotKind = "connection def dot"
	dependencyDotKind    = "dependency dot"
)

// isDotKind reports whether a kind is an n-ary dot, drawn as a filled dot with
// its name beside it in every form.
func isDotKind(kind string) bool {
	return kind == connectionDotKind || kind == connectionDefDotKind || kind == dependencyDotKind
}

// naryDot is the dot node of the n-ary element sym, labelled name (the
// notation's optional cdot-label; "" draws none).
func (r *Renderer) naryDot(id, kind, name string, view, sym *symbols.Symbol, out *Rendering) *Node {
	return &Node{ID: id, Kind: kind, Name: name, Origin: symbolOrigin(sym),
		Geometry: r.geometryOf(view, sym, out), Style: r.styleOf(view, sym, out)}
}

// nary reports whether a connector or dependency with the given number of
// ends is drawn as a dot rather than as one line.
func nary(ends int) bool { return ends > 2 }

// The arrows of an item box on a connection line (§8.2.3.16): a flow's item
// arrow has a filled head, a message's an open one, pointing the way the item goes.
const (
	flowArrowRight    = "—▶"
	flowArrowLeft     = "◀—"
	messageArrowRight = "—>"
	messageArrowLeft  = "<—"
	arrowRight        = "right"
	arrowLeft         = "left"
)

// itemArrow is the arrow line under an item box's text, "" for a node that is
// no item box.
func itemArrow(node *Node) string {
	switch {
	case node.Arrow == "":
		return ""
	case node.Kind == "message" && node.Arrow == arrowLeft:
		return messageArrowLeft
	case node.Kind == "message":
		return messageArrowRight
	case node.Arrow == arrowLeft:
		return flowArrowLeft
	}
	return flowArrowRight
}

// hasDot reports whether an n-ary dot is drawn among the nodes or beneath them.
func hasDot(nodes []*Node) bool {
	for _, node := range nodes {
		if isDotKind(node.Kind) || hasDot(node.Children) {
			return true
		}
	}
	return false
}

// itemBox reports whether a node is an item box on a connection line.
func itemBox(node *Node) bool { return node.Arrow != "" }
