package view

import (
	"errors"
	"fmt"
	"strings"
)

// DrawingStyle names the look the DOT form draws a diagram in: the Pilot
// "Standard B&W" default, or the look of a diagram drawn by Cameo Systems
// Modeler, for a document migrated from one. A style is independent of the
// palette: a palette recolours the nodes of whichever style is drawn.
type DrawingStyle string

const (
	// StylePilot is the Pilot "Standard B&W" look: Helvetica text, white
	// fills, thin #181818 lines, no diagram frame. The empty style is this one.
	StylePilot DrawingStyle = "pilot"
	// StyleCameo is the look of a Cameo Systems Modeler diagram: a frame with
	// the `kind [Type] Owner [ Name ]` header tab, Arial text, the pale yellow,
	// green and orange gradient fills Cameo gives states, actions and blocks,
	// thin dark borders, and notes with a folded corner and a dashed anchor.
	StyleCameo DrawingStyle = "cameo"
)

// DrawingStyles are the styles the DOT form can be asked for, in the order
// they are offered.
func DrawingStyles() []DrawingStyle { return []DrawingStyle{StylePilot, StyleCameo} }

// ParseDrawingStyle reports the style name names, and whether it names one.
// The empty name is the default, StylePilot.
func ParseDrawingStyle(name string) (DrawingStyle, bool) {
	if name == "" {
		return StylePilot, true
	}
	for _, style := range DrawingStyles() {
		if string(style) == name {
			return style, true
		}
	}
	return "", false
}

// DrawingStyleNames spells the styles as a list, for help and error text.
func DrawingStyleNames() string {
	names := make([]string, 0, len(DrawingStyles()))
	for _, style := range DrawingStyles() {
		names = append(names, string(style))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownDrawingStyle is a style name that names none. UnknownDrawingStyleError
// wraps it.
var ErrUnknownDrawingStyle = errors.New("unknown drawing style")

// UnknownDrawingStyleError is a style asked for by a name no style has; it
// names the styles there are.
type UnknownDrawingStyleError struct {
	Name string
}

func (e *UnknownDrawingStyleError) Error() string {
	return fmt.Sprintf("unknown drawing style %q; the styles are %s", e.Name, DrawingStyleNames())
}

func (e *UnknownDrawingStyleError) Unwrap() error { return ErrUnknownDrawingStyle }

// check is the UnknownDrawingStyleError of a style no registry entry has; the
// empty style and every registered one pass.
func (s DrawingStyle) check() error {
	if _, ok := ParseDrawingStyle(string(s)); !ok {
		return &UnknownDrawingStyleError{Name: string(s)}
	}
	return nil
}

// styleNotice is the notice a form that draws no style writes for one asked
// for, so the request is not dropped silently.
func styleNotice(style DrawingStyle) string {
	return fmt.Sprintf("style %s; only the DOT form draws a diagram in a style", style)
}

// TakesStyle reports whether the form draws a diagram in a DrawingStyle.
func (f Form) TakesStyle() bool { return f == FormDot }

// countStyled is how many nodes and edges of the rendering carry a Style.
func (r *Rendering) countStyled() (nodes, edges int) {
	var walk func(node *Node)
	walk = func(node *Node) {
		if node.Style != nil {
			nodes++
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	for _, edge := range r.Edges {
		if edge.Style != nil {
			edges++
		}
	}
	return nodes, edges
}

// visualNotices are the notices a form writes for the Styles, Notes and Pictures the
// rendering carries and it does not draw, so neither is dropped silently: what
// says which part of a Style the form leaves out; notes are left out of the
// notice when the form writes them itself.
func (r *Rendering) visualNotices(what string, withNotes bool) []string {
	var notices []string
	if nodes, edges := r.countStyled(); nodes+edges > 0 {
		notices = append(notices, fmt.Sprintf("%d styled node(s) and %d styled edge(s); %s; the dot form draws every Style", nodes, edges, what))
	}
	if withNotes && len(r.Notes) > 0 {
		notices = append(notices, fmt.Sprintf("%d note(s); the dot form draws notes", len(r.Notes)))
	}
	if withNotes && len(r.Pictures) > 0 {
		notices = append(notices, r.pictureNotice())
	}
	return notices
}

// pictureNotice names the pictures a form does not draw, with their bounds.
func (r *Rendering) pictureNotice() string {
	placed := make([]string, len(r.Pictures))
	for i, p := range r.Pictures {
		placed[i] = fmt.Sprintf("%s at (%s, %s) size %s×%s", p.Location, formatCoord(p.X), formatCoord(p.Y), formatCoord(p.Width), formatCoord(p.Height))
	}
	return fmt.Sprintf("%d picture(s) not drawn: %s; the dot form draws pictures", len(r.Pictures), strings.Join(placed, ", "))
}

// What each form leaves out of a Style, for visualNotices.
const (
	noFontOrEdgeStyle = "a node's font and an edge's Style are not drawn"
	noStyleInText     = "colours and fonts are not written"
)

// The Cameo look, measured from pages of a Cameo-published document; the table
// in docs/project/view-rendering-forms.md records the measurements. Each fill
// is a horizontal gradient, left to right colour; each pen is the border colour
// under the anti-aliasing. Cameo's drop shadow is not drawn: Graphviz has none.
const (
	cameoFontName   = "Arial"
	cameoFontSize   = 11
	cameoSmallPts   = 9
	cameoTextColor  = "#424242"
	cameoLineColor  = "#5B5B59"
	cameoEdgeColor  = "#424242"
	cameoFrameColor = "#5B5B59"
	cameoNoteFill   = "#FFFFFF"
	cameoStateFill  = "#FFFFCC:#FFFFF2"
	cameoActionFill = "#E1E1C3:#F7F7EF"
	cameoActionLine = "#424242"
	cameoBlockFill  = "#FFCC99:#FFFAD4"
	cameoBlockLine  = "#99795C"
)

// A Cameo fork or join bar drawn with no stated box, in points.
const (
	cameoBarWidth  = 60
	cameoBarHeight = 5
)

// cameoFill is the Cameo gradient a node of the kind is filled with, and its
// pen: states pale yellow, actions and the control nodes pale green, everything
// else a block's orange.
func cameoFill(kind string) (fill, pen string) {
	if controlKinds[kind] {
		return cameoActionFill, cameoActionLine
	}
	switch paletteFamily(kind) {
	case "state":
		return cameoStateFill, cameoLineColor
	case "action", "flow":
		return cameoActionFill, cameoActionLine
	}
	return cameoBlockFill, cameoBlockLine
}

// cameoRounded reports whether Cameo draws a node of the kind with rounded
// corners: states, actions and regions are; blocks, parts and ports square.
func cameoRounded(kind string) bool {
	switch paletteFamily(kind) {
	case "state", "action":
		return true
	}
	return kind == "region"
}

// cameoFrameKind is the diagram kind Cameo's frame header leads with.
func cameoFrameKind(kind Kind) string {
	switch kind {
	case KindTree:
		return "bdd"
	case KindInterconnection:
		return "ibd"
	case KindState:
		return "stm"
	case KindAction:
		return "act"
	}
	return string(kind)
}

// cameoFrameType is the bracketed type of the diagram's context element in the
// frame header, `[State Machine]` for a state definition.
func cameoFrameType(kind string) string {
	switch definitionKeyword(kind) {
	case "state":
		return "State Machine"
	case "action":
		return "Activity"
	case "part":
		return "Block"
	case "":
		return ""
	}
	words := strings.Fields(definitionKeyword(kind))
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
