package view

import (
	"fmt"
	"slices"
	"strings"
)

// PlantUML is the PlantUML form of a rendering: a class diagram of a tree, a
// nested rectangle diagram of an interconnection, a state diagram of a state
// or action rendering, and a sequence diagram of a sequence, in the Standard
// B&W style with its rules written into the file. What the rendering could
// not represent is written as `'` comments, so no notice is lost.
func (r *Rendering) PlantUML() (string, error) {
	return r.PlantUMLWith(Options{})
}

// PlantUMLWith is the PlantUML form written with options: drawn in the stated
// direction (PlantUML draws top to bottom or left to right, so a reversed
// direction takes its nearest and is noted as not represented; the empty
// direction leaves PlantUML's default) and filled from the stated palette by
// keyword family, as the DOT form is. Placement is written as comments,
// PlantUML having no absolute positions; for pinned positions use the DOT form.
func (r *Rendering) PlantUMLWith(options Options) (string, error) {
	if !r.Kind.SupportsForm(FormPlantUML) {
		return "", &WrongFormError{Form: FormPlantUML, Kind: r.Kind, View: r.View}
	}
	if err := options.Palette.check(); err != nil {
		return "", err
	}
	w := &plantumlWriter{borders: r.Kind != KindSequence, fills: familyFills{palette: options.Palette, tree: r.Kind == KindTree}}
	for _, root := range r.Roots {
		w.fills.collect(root)
	}
	var notices []string
	direction, reversed := plantumlDirection(options.Direction)
	if !r.Kind.SupportsDirection() {
		direction = ""
	} else if reversed {
		notices = append(notices, fmt.Sprintf("direction %s; PlantUML draws no reversed direction, so the diagram reads %s",
			options.Direction, strings.TrimSuffix(direction, " direction")))
	}
	if placed, routed := r.countGeometry(); placed+routed > 0 || r.Canvas != nil {
		notices = append(notices, fmt.Sprintf("%d positioned node(s) and %d route(s) kept as comments; PlantUML pins no position, the dot form does", placed, routed))
	}
	b := &w.b
	b.WriteString("@startuml\n")
	if r.View == "" {
		fmt.Fprintf(b, "' %s rendering", r.Kind)
	} else {
		fmt.Fprintf(b, "' %s — %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	for _, notice := range slices.Concat(r.Notices, notices) {
		fmt.Fprintf(b, "' not represented: %s\n", notice)
	}
	w.writeStyle()
	if direction != "" {
		b.WriteString(direction + "\n")
	}
	r.writeGeometryComments(b, "'")
	switch r.Kind {
	case KindTree:
		w.writeClassDiagram(r)
	case KindInterconnection:
		w.writeRectangleDiagram(r)
	case KindState, KindAction:
		w.writeStateDiagram(r)
	case KindSequence:
		w.writeSequenceDiagram(r)
	}
	b.WriteString("@enduml\n")
	return b.String(), nil
}

// plantumlWriter holds what one rendering's PlantUML form needs across nodes and edges.
type plantumlWriter struct {
	b       strings.Builder
	borders bool        // whether a filled node's border takes the family colour; a participant's cannot
	fills   familyFills // the palette fills, by keyword family
}

// countGeometry counts the nodes a Geometry positions and the edges with a route.
func (r *Rendering) countGeometry() (placed, routed int) {
	var walk func(node *Node)
	walk = func(node *Node) {
		if node.Geometry != nil {
			placed++
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	for _, edge := range r.Edges {
		if len(edge.Route) > 0 {
			routed++
		}
	}
	return placed, routed
}

// plantumlDirection is the direction statement drawing in direction, or its
// nearest when PlantUML has none for it: a reversed direction reads forwards,
// which reversed reports. The empty direction states nothing.
func plantumlDirection(direction Direction) (statement string, reversed bool) {
	switch direction {
	case DirectionTopBottom:
		return "top to bottom direction", false
	case DirectionBottomTop:
		return "top to bottom direction", true
	case DirectionLeftRight:
		return "left to right direction", false
	case DirectionRightLeft:
		return "left to right direction", true
	}
	return "", false
}

// The Standard B&W style, after the sysmlbw PlantUML skin: sans-serif 14pt
// black text, white fills, thin #181818 lines, no shadows, definitions square
// and usages rounded, edge text a point smaller than node text, labels wrapped
// at 300 pixels.
const (
	plantumlFontName       = "SansSerif"
	plantumlFontSize       = 14
	plantumlEdgeFontSize   = 13
	plantumlLineColor      = "#181818"
	plantumlLineThickness  = "0.5"
	plantumlGroupThickness = "1.5"
	plantumlUsageRadius    = 20
	plantumlNoteColor      = "#FEFFDD"
	plantumlWrapWidth      = 300
)

// plantumlKeywordFontSize is the font size of the guillemet keyword line,
// under the name as the skin sets a stereotype.
const plantumlKeywordFontSize = 10

// writeStyle writes the style block every file carries: the skin's rules on
// every element, the UML black of the start and end dots and the fork and join
// bars, the corner radius a usage's stereotype rounds, the heavier border of a
// package and the dashed one of a region. Stereotypes are hidden: the label
// carries the keyword line.
func (w *plantumlWriter) writeStyle() {
	b := &w.b
	b.WriteString("<style>\n")
	fmt.Fprintf(b, "root {\n  BackGroundColor white\n  FontName %s\n  FontSize %d\n  FontColor black\n  LineColor %s\n  HorizontalAlignment left\n}\n",
		plantumlFontName, plantumlFontSize, plantumlLineColor)
	fmt.Fprintf(b, "element {\n  BackGroundColor white\n  LineColor %s\n  LineThickness %s\n  RoundCorner 0\n  Shadowing 0.0\n}\n",
		plantumlLineColor, plantumlLineThickness)
	b.WriteString("start, end, activityBar {\n  BackGroundColor black\n}\n")
	fmt.Fprintf(b, "arrow {\n  LineColor %s\n  LineThickness 1\n  FontSize %d\n}\n", plantumlLineColor, plantumlEdgeFontSize)
	fmt.Fprintf(b, "note {\n  BackGroundColor %s\n  FontSize %d\n}\n", plantumlNoteColor, plantumlEdgeFontSize)
	fmt.Fprintf(b, ".%s {\n  RoundCorner %d\n}\n", plantumlUsageStereotype, plantumlUsageRadius)
	fmt.Fprintf(b, ".%s {\n  LineThickness %s\n}\n", plantumlPackageStereotype, plantumlGroupThickness)
	b.WriteString(".region {\n  LineStyle 4\n}\n")
	b.WriteString("</style>\n")
	fmt.Fprintf(b, "skinparam wrapWidth %d\n", plantumlWrapWidth)
	b.WriteString("hide stereotype\n")
}

// plantumlUsageStereotype and plantumlPackageStereotype are the stereotypes a
// usage and a package carry beside their keyword, so one style rule each
// rounds the usages and thickens the packages.
const (
	plantumlUsageStereotype   = "usage"
	plantumlPackageStereotype = "package"
)

// writeClassDiagram writes a tree as a class diagram: every node a class, and
// containment an edge from a node to each of its children, as the Mermaid and
// DOT trees draw it. Empty compartments and the class circle are hidden.
func (w *plantumlWriter) writeClassDiagram(r *Rendering) {
	b := &w.b
	b.WriteString("hide circle\nhide empty members\n")
	if r.Empty() {
		fmt.Fprintf(b, "class %s as empty\n", plantumlQuote(r.EmptyReason()))
		return
	}
	for _, root := range r.Roots {
		w.writeClassNode(root)
	}
	for _, edge := range r.Edges {
		w.writeEdge(edge)
	}
}

// writeClassNode writes one class and, in a tree, its children with the
// containment edge to each.
func (w *plantumlWriter) writeClassNode(node *Node) {
	fmt.Fprintf(&w.b, "class %s as %s%s\n", plantumlQuote(plantumlLabel(node)), node.ID, w.decoration(node))
	for _, child := range node.Children {
		w.writeClassNode(child)
		fmt.Fprintf(&w.b, "%s -- %s\n", node.ID, child.ID)
	}
}

// writeRectangleDiagram writes an interconnection as nested rectangles: a node
// with children is a rectangle block holding them, a connection an undirected
// heavy line, a flow a dashed arrow.
func (w *plantumlWriter) writeRectangleDiagram(r *Rendering) {
	if r.Empty() {
		fmt.Fprintf(&w.b, "rectangle %s as empty\n", plantumlQuote(r.EmptyReason()))
		return
	}
	for _, root := range r.Roots {
		w.writeRectangleNode(root, 0)
	}
	for _, edge := range r.Edges {
		w.writeEdge(edge)
	}
}

// writeRectangleNode writes one rectangle, a block of its children when it has any.
func (w *plantumlWriter) writeRectangleNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(&w.b, "%srectangle %s as %s%s", indent, plantumlQuote(plantumlLabel(node)), node.ID, w.decoration(node))
	if len(node.Children) == 0 {
		w.b.WriteString("\n")
		return
	}
	w.b.WriteString(" {\n")
	for _, child := range node.Children {
		w.writeRectangleNode(child, depth+1)
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeStateDiagram writes a state or action rendering as a state diagram:
// bodies are composite states, entry transitions leave the `[*]` marker of
// their body, and every other edge is a transition. An action graph takes the
// same grammar, its control nodes as pseudostates, since PlantUML's activity
// grammar is procedural and cannot hold an arbitrary graph.
func (w *plantumlWriter) writeStateDiagram(r *Rendering) {
	b := &w.b
	b.WriteString("hide empty description\n")
	if r.Empty() {
		fmt.Fprintf(b, "state %s as empty\n", plantumlQuote(r.EmptyReason()))
		return
	}
	starts := map[string][]Edge{}
	for _, root := range r.Roots {
		collectStarts(root, starts)
	}
	for _, edge := range r.Edges {
		if _, ok := starts[edge.From]; ok {
			starts[edge.From] = append(starts[edge.From], edge)
		}
	}
	for _, root := range r.Roots {
		w.writeStateNode(root, 0, starts)
	}
	for _, edge := range r.Edges {
		if _, ok := starts[edge.From]; ok {
			continue
		}
		w.writeEdge(edge)
	}
}

// writeStateNode writes one state and its substates. A body's start is the
// `[*]` marker inside that state, so its edges are written there after the substates.
func (w *plantumlWriter) writeStateNode(node *Node, depth int, starts map[string][]Edge) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(&w.b, "%sstate %s as %s%s", indent, plantumlQuote(plantumlLabel(node)), node.ID, w.decoration(node))
	if len(node.Children) == 0 {
		w.b.WriteString("\n")
		return
	}
	w.b.WriteString(" {\n")
	for _, child := range node.Children {
		if child.Kind != startKind {
			w.writeStateNode(child, depth+1, starts)
		}
	}
	for _, child := range node.Children {
		for _, edge := range starts[child.ID] {
			w.writeArrow(indent+"  ", "[*]", edge.To, plantumlArrow(edge.Kind), edge.Label)
		}
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeSequenceDiagram writes a sequence rendering: one participant per
// lifeline, declared before the messages, then the messages in the order the
// rendering settled on. A participant is filled under a palette like any usage.
func (w *plantumlWriter) writeSequenceDiagram(r *Rendering) {
	b := &w.b
	if r.Empty() {
		fmt.Fprintf(b, "participant %s as empty\n", plantumlQuote(r.EmptyReason()))
		return
	}
	for _, node := range r.Roots {
		fmt.Fprintf(b, "participant %s as %s%s\n", plantumlQuote(plantumlLabel(node)), node.ID, w.decoration(node))
	}
	for _, edge := range r.Edges {
		w.writeArrow("", edge.From, edge.To, "->", edge.Label)
	}
}

// writeEdge writes one edge as its kind's arrow, with its label when it carries one.
func (w *plantumlWriter) writeEdge(edge Edge) {
	w.writeArrow("", edge.From, edge.To, plantumlArrow(edge.Kind), edge.Label)
}

// writeArrow writes one arrow statement between two aliases.
func (w *plantumlWriter) writeArrow(indent, from, to, arrow, label string) {
	if label == "" {
		fmt.Fprintf(&w.b, "%s%s %s %s\n", indent, from, arrow, to)
		return
	}
	fmt.Fprintf(&w.b, "%s%s %s %s : %s\n", indent, from, arrow, to, plantumlText(label))
}

// plantumlArrow is how an edge of each kind is drawn: a connection as the
// Pilot's heavy undirected connector, a flow dashed, every other edge a plain arrow.
func plantumlArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "-[thickness=3]-"
	case EdgeFlow:
		return "-[dashed]->"
	}
	return "-->"
}

// decoration is what follows a node's alias: its stereotypes — the keyword,
// then the shape stereotype the style selects on when the keyword is not one
// itself — and its fill, with the border in the family colour, under a palette.
func (w *plantumlWriter) decoration(node *Node) string {
	var out strings.Builder
	if pseudostate := plantumlPseudostates[node.Kind]; pseudostate != "" {
		// PlantUML draws a pseudostate only when its stereotype stands alone.
		fmt.Fprintf(&out, " <<%s>>", pseudostate)
		return out.String()
	}
	if node.Kind != "" {
		fmt.Fprintf(&out, " <<%s>>", node.Kind)
	}
	if shape := plantumlShapeStereotype(node); shape != "" && shape != node.Kind {
		fmt.Fprintf(&out, " <<%s>>", shape)
	}
	if w.fills.filled(node) {
		out.WriteString(" " + w.fills.fill(node))
		if w.borders {
			out.WriteString(";line:" + strings.TrimPrefix(w.fills.color(node), "#"))
		}
	}
	return out.String()
}

// plantumlPseudostates are PlantUML's pseudostate stereotypes by the control
// kinds they draw: the initial and final dots, the fork and join bars, the
// choice diamond (a merge and a junction too, PlantUML having no round
// junction), and the history circles.
var plantumlPseudostates = map[string]string{
	"initial": "start", "final": "end", "fork": "fork", "join": "join",
	"decision": "choice", "choice": "choice", "merge": "choice", "junction": "choice",
	"shallow history": "history", "deep history": "history*",
}

// plantumlShapeStereotype is the stereotype the style block shapes a node by:
// `package` for a package's heavier border, `usage` for a usage's rounded
// corners; a definition or an orthogonal region keeps the element rules.
func plantumlShapeStereotype(node *Node) string {
	switch {
	case slices.Contains(strings.Fields(node.Kind), plantumlPackageStereotype):
		return plantumlPackageStereotype
	case node.Kind == "region", isDefinitionKind(node.Kind):
		return ""
	}
	return plantumlUsageStereotype
}

// plantumlLabel is a node's label ready to quote: the name line bold, the
// keyword line italic at the skin's stereotype size, every line escaped.
func plantumlLabel(node *Node) string {
	lines := labelLines(node)
	parts := []string{"**" + plantumlText(lines[0]) + "**"}
	for _, line := range lines[1:] {
		parts = append(parts, plantumlText(line))
	}
	if node.Name != "" {
		parts[1] = fmt.Sprintf("<size:%d>//%s//</size>", plantumlKeywordFontSize, parts[1])
	}
	return strings.Join(parts, `\n`)
}

// plantumlQuote wraps text in double quotes for a PlantUML display name.
func plantumlQuote(text string) string {
	return `"` + text + `"`
}

// plantumlText writes text so PlantUML shows it as it is. A quote, a backslash,
// an angle bracket and the creole escape `~` become `<U+XXXX>` escapes, as does
// each of a run of the characters creole reads doubled (`**`, `//`, `__`, `--`,
// `[[`, `]]`); a newline becomes `\n`.
func plantumlText(text string) string {
	runes := []rune(text)
	var out strings.Builder
	for i, c := range runes {
		switch {
		case c == '\n':
			out.WriteString(`\n`)
		case strings.ContainsRune(`"\<>~`, c),
			strings.ContainsRune("*/_-[]", c) && (i > 0 && runes[i-1] == c || i+1 < len(runes) && runes[i+1] == c):
			fmt.Fprintf(&out, "<U+%04X>", c)
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}
