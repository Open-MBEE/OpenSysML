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
// keyword family, as the DOT form is. A rendering some Layout positions draws
// the nodes the DOT form draws: the placed ones, and the unplaced ones too under
// UnplacedStrip. Placement itself is written as comments, PlantUML having no
// absolute positions; for pinned positions use the DOT form.
func (r *Rendering) PlantUMLWith(options Options) (string, error) {
	if !r.supportsForm(FormPlantUML) {
		return "", r.wrongFormError(FormPlantUML)
	}
	if err := options.Palette.check(); err != nil {
		return "", err
	}
	if err := options.Unplaced.check(); err != nil {
		return "", err
	}
	if err := options.Ports.check(); err != nil {
		return "", err
	}
	if r.Run && r.Kind == KindTimeline {
		return r.runTimelinePlantUML(options), nil
	}
	r = r.settleUnplaced(options.Unplaced, FormPlantUML)
	w := &plantumlWriter{kind: r.Kind, borders: r.Kind.paletteBorders(), fills: familyFills{palette: options.Palette, tree: r.Kind == KindTree},
		labels: labelsOf(r.Roots, false, nil), ports: r.portView(options.Ports), links: options.Links,
		writtenPorts: make(map[string]struct{})}
	for _, root := range r.Roots {
		w.fills.collect(root)
	}
	var notices []string
	direction, reversed := plantumlDirection(options.Direction)
	if options.Direction == "" && r.Kind == KindCase {
		direction, reversed = plantumlDirection(DirectionLeftRight)
	}
	if !r.Kind.SupportsDirection() {
		direction = ""
	} else if reversed {
		notices = append(notices, fmt.Sprintf("direction %s; PlantUML draws no reversed direction, so the diagram reads %s",
			options.Direction, strings.TrimSuffix(direction, " direction")))
	}
	if placed, routed := r.countGeometry(); placed+routed > 0 || r.Canvas != nil {
		notices = append(notices, fmt.Sprintf("%d positioned node(s) and %d route(s) kept as comments; PlantUML pins no position, the dot form does", placed, routed))
	}
	if options.Style != "" && options.Style != StylePilot {
		notices = append(notices, styleNotice(options.Style))
	}
	if r.Kind == KindMixed {
		notices = append(notices, w.mixedContainmentNotices(r.Roots)...)
	}
	if r.Kind == KindAction {
		if pins := r.undrawnPins(func(*Node, Port) bool { return false }); len(pins) > 0 {
			notices = append(notices, fmt.Sprintf("%d pin(s) not drawn (%s); PlantUML's state grammar has no pin, so the edges name them",
				len(pins), strings.Join(pins, ", ")))
		}
	}
	notices = append(notices, r.visualNotices(noFontOrEdgeStyle, true)...)
	b := &w.b
	b.WriteString("@startuml\n")
	if r.Run {
		fmt.Fprintf(b, "' run — %s rendering", r.Kind)
	} else if r.View == "" {
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
	case KindTree, KindRequirement, KindDefinition, KindPackage:
		w.writeClassDiagram(r)
	case KindInterconnection:
		w.writeRectangleDiagram(r)
	case KindCase, KindMixed:
		w.writeCaseMixedDiagram(r, r.Kind == KindMixed)
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
	b            strings.Builder
	kind         Kind
	borders      bool        // whether a filled node's border takes the family colour; a participant's cannot
	fills        familyFills // the palette fills, by keyword family
	labels       labeller    // the node labels, headed relative to the roots' namespace
	ports        portView    // the ports drawn of each node, and how they are named
	links        Links
	writtenPorts map[string]struct{}
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
	if r.blank() {
		fmt.Fprintf(b, "class %s as empty\n", plantumlQuote(r.blankReason(FormPlantUML)))
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
	fmt.Fprintf(&w.b, "class %s as %s%s\n", plantumlQuote(w.plantumlLabel(node)), node.ID, w.decoration(node))
	for _, child := range node.Children {
		w.writeClassNode(child)
		fmt.Fprintf(&w.b, "%s -- %s\n", node.ID, child.ID)
	}
}

// writeRectangleDiagram writes an interconnection as nested rectangles: a node
// with children or drawn ports is a rectangle block holding them, a port a
// `port` on the rectangle's border — named alone under the minimal display,
// `name : Type` under the full — a connection an undirected heavy line, a flow
// a dashed arrow, each ending at the port it names.
func (w *plantumlWriter) writeRectangleDiagram(r *Rendering) {
	if r.blank() {
		fmt.Fprintf(&w.b, "rectangle %s as empty\n", plantumlQuote(r.blankReason(FormPlantUML)))
		return
	}
	for _, root := range r.Roots {
		w.writeRectangleNode(root, 0)
	}
	for _, edge := range r.Edges {
		w.writeArrowEdge(edge, portOr(edge.FromPort, edge.From), portOr(edge.ToPort, edge.To), false)
	}
}

// portOr is the alias an edge ends at: the port when it names one, else the node.
func portOr(port, node string) string {
	if port != "" {
		return port
	}
	return node
}

// writeRectangleNode writes one rectangle, a block of its ports and children
// when it has any.
func (w *plantumlWriter) writeRectangleNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(&w.b, "%srectangle %s as %s%s", indent, plantumlQuote(w.plantumlLabel(node)), node.ID, w.decoration(node))
	ports := w.ports.of(node)
	if len(node.Children) == 0 && len(ports) == 0 {
		w.b.WriteString("\n")
		return
	}
	w.b.WriteString(" {\n")
	for _, port := range ports {
		fmt.Fprintf(&w.b, "%s  port %s as %s\n", indent, plantumlQuote(plantumlText(w.ports.pinLabel(port))), port.ID)
	}
	for _, child := range node.Children {
		w.writeRectangleNode(child, depth+1)
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeCaseMixedDiagram writes roots and relationships in the rectangle dialect.
func (w *plantumlWriter) writeCaseMixedDiagram(r *Rendering, mixed bool) {
	if r.blank() {
		fmt.Fprintf(&w.b, "rectangle %s as empty\n", plantumlQuote(r.blankReason(FormPlantUML)))
		return
	}
	for _, root := range r.Roots {
		w.writeCaseMixedNode(root, 0, mixed)
	}
	for _, edge := range r.Edges {
		w.writeEdge(edge)
	}
}

// mixedContainmentNotices reports case and actor children PlantUML cannot nest.
func (w *plantumlWriter) mixedContainmentNotices(roots []*Node) []string {
	var notices []string
	var walk func(*Node)
	walk = func(node *Node) {
		if caseNodeKind(node.Kind) || node.Kind == "actor" {
			for _, child := range node.Children {
				if caseNodeKind(child.Kind) {
					continue
				}
				element := "usecase"
				if node.Kind == "actor" {
					element = "actor"
				}
				name := strings.TrimSpace(node.Kind + " " + w.labels.name(node))
				notices = append(notices, fmt.Sprintf(
					"non-case children of %s are drawn flat; PlantUML %s elements cannot contain nodes", name, element))
				break
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return notices
}

// writeCaseMixedContainer writes a mixed rectangle and its ports and children.
func (w *plantumlWriter) writeCaseMixedContainer(node *Node, depth int, mixed bool) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(&w.b, "%srectangle %s as %s%s {\n", indent, plantumlQuote(w.plantumlLabel(node)), node.ID, w.decoration(node))
	for _, port := range w.ports.of(node) {
		w.writtenPorts[port.ID] = struct{}{}
		fmt.Fprintf(&w.b, "%s  port %s as %s\n", indent, plantumlQuote(plantumlText(w.ports.pinLabel(port))), port.ID)
	}
	for _, child := range node.Children {
		w.writeCaseMixedNode(child, depth+1, mixed)
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeCaseMixedNode writes one case/mixed node using native PlantUML elements.
func (w *plantumlWriter) writeCaseMixedNode(node *Node, depth int, mixed bool) {
	indent := strings.Repeat("  ", depth)
	label := plantumlQuote(w.plantumlLabel(node))
	switch {
	case slices.Contains(strings.Fields(node.Kind), "package"):
		fmt.Fprintf(&w.b, "%spackage %s as %s%s {\n", indent, label, node.ID, w.decoration(node))
		for _, child := range node.Children {
			w.writeCaseMixedNode(child, depth+1, mixed)
		}
		fmt.Fprintf(&w.b, "%s}\n", indent)
	case caseNodeKind(node.Kind):
		fmt.Fprintf(&w.b, "%susecase %s as %s%s\n", indent, label, node.ID, w.decoration(node))
		for _, child := range node.Children {
			w.writeCaseMixedNode(child, depth+1, mixed)
		}
	case node.Kind == "actor":
		fmt.Fprintf(&w.b, "%sactor %s as %s%s\n", indent, label, node.ID, w.decoration(node))
		for _, child := range node.Children {
			w.writeCaseMixedNode(child, depth+1, mixed)
		}
	case mixed && (len(node.Children) > 0 || len(w.ports.of(node)) > 0 && !controlKinds[node.Kind]):
		w.writeCaseMixedContainer(node, depth, mixed)
	case node.Kind == "subject":
		fmt.Fprintf(&w.b, "%srectangle %s as %s%s\n", indent, label, node.ID, w.decoration(node))
	case node.Kind == "objective":
		if url, ok := w.links.URL(node.Origin); ok {
			note := strings.ReplaceAll(w.plantumlLabel(node), `\n`, "\n")
			fmt.Fprintf(&w.b, "%snote as %s%s\n", indent, node.ID, w.noteDecoration(node))
			for _, line := range strings.Split(note, "\n") {
				if line == "" {
					fmt.Fprintf(&w.b, "%s  \n", indent)
					continue
				}
				if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") && len(line) > 4 {
					line = "**[[" + url + " " + plantumlNoteLinkText(line[2:len(line)-2]) + "]]**"
				} else {
					line = "[[" + url + " " + plantumlNoteLinkText(line) + "]]"
				}
				fmt.Fprintf(&w.b, "%s  %s\n", indent, line)
			}
			fmt.Fprintf(&w.b, "%send note\n", indent)
			return
		}
		fmt.Fprintf(&w.b, "%snote %s as %s%s\n", indent, label, node.ID, w.noteDecoration(node))
	case mixed && controlKinds[node.Kind]:
		fmt.Fprintf(&w.b, "%scircle %s as %s%s\n", indent, label, node.ID, w.decoration(node))
	default:
		fmt.Fprintf(&w.b, "%srectangle %s as %s%s\n", indent, label, node.ID, w.decoration(node))
		for _, child := range node.Children {
			w.writeCaseMixedNode(child, depth+1, mixed)
		}
	}
}

// writeStateDiagram writes a state or action rendering as a state diagram:
// bodies are composite states, entry transitions leave the `[*]` marker of
// their body, and every other edge is a transition. An action graph takes the
// same grammar, its control nodes as pseudostates, since PlantUML's activity
// grammar is procedural and cannot hold an arbitrary graph.
func (w *plantumlWriter) writeStateDiagram(r *Rendering) {
	b := &w.b
	b.WriteString("hide empty description\n")
	if r.blank() {
		fmt.Fprintf(b, "state %s as empty\n", plantumlQuote(r.blankReason(FormPlantUML)))
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
	fmt.Fprintf(&w.b, "%sstate %s as %s%s", indent, plantumlQuote(w.plantumlLabel(node)), node.ID, w.decoration(node))
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
			w.writeArrow(indent+"  ", "[*]", edge.To, plantumlArrow(w.kind, edge.Kind), edge.Label, Origin{}, false)
		}
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeSequenceDiagram writes a sequence rendering: one participant per
// lifeline, declared before the messages, then the messages in the order the
// rendering settled on. A participant is filled under a palette like any usage.
func (w *plantumlWriter) writeSequenceDiagram(r *Rendering) {
	b := &w.b
	if r.blank() {
		fmt.Fprintf(b, "participant %s as empty\n", plantumlQuote(r.blankReason(FormPlantUML)))
		return
	}
	for _, node := range r.Roots {
		label := w.plantumlLabel(node)
		if r.Run {
			label = plantumlText(runParticipantLabel(node))
		}
		fmt.Fprintf(b, "participant %s as %s%s\n", plantumlQuote(label), node.ID, w.decoration(node))
	}
	for _, edge := range r.Edges {
		w.writeArrowEdge(edge, edge.From, edge.To, true)
	}
}

// writeEdge writes one edge as its kind's arrow, with its label when it carries one.
func (w *plantumlWriter) writeEdge(edge Edge) {
	from, to := edge.From, edge.To
	if w.kind == KindMixed {
		if _, written := w.writtenPorts[edge.FromPort]; written {
			from = edge.FromPort
		}
		if _, written := w.writtenPorts[edge.ToPort]; written {
			to = edge.ToPort
		}
	}
	w.writeArrowEdge(edge, from, to, false)
}

// writeArrow writes one arrow statement between two aliases.
func (w *plantumlWriter) writeArrow(indent, from, to, arrow, label string, origin Origin, sequence bool) {
	link := ""
	if url, ok := w.links.URL(origin); ok {
		link = " [[" + url + "]]"
	}
	if label == "" {
		if link != "" && !sequence {
			fmt.Fprintf(&w.b, "%s%s %s %s :%s\n", indent, from, arrow, to, link)
			return
		}
		fmt.Fprintf(&w.b, "%s%s %s %s%s\n", indent, from, arrow, to, link)
		return
	}
	edgeText := plantumlEdgeText(w.kind, label)
	if sequence && link != "" {
		fmt.Fprintf(&w.b, "%s%s %s %s :%s %s\n", indent, from, arrow, to, link, edgeText)
		return
	}
	fmt.Fprintf(&w.b, "%s%s %s %s : %s%s\n", indent, from, arrow, to, edgeText, link)
}

func (w *plantumlWriter) writeArrowEdge(edge Edge, from, to string, sequence bool) {
	arrow := plantumlArrow(w.kind, edge.Kind)
	if sequence {
		arrow = "->"
	}
	w.writeArrow("", from, to, arrow, edge.Label, edge.Origin, sequence)
}

// plantumlEdgeText preserves case and mixed relationship guillemets in labels.
func plantumlEdgeText(kind Kind, text string) string {
	text = plantumlText(text)
	if kind == KindCase || kind == KindMixed {
		text = strings.NewReplacer("«", "<U+00AB>", "»", "<U+00BB>").Replace(text)
	}
	return text
}

// plantumlStyleColor is a Style's colours as PlantUML's inline colour,
// `#fill;line:RRGGBB;text:RRGGBB`; with no fill it opens `#;`.
func plantumlStyleColor(style *Style) string {
	color := "#" + strings.TrimPrefix(style.Fill, "#")
	if style.Line != "" {
		color += ";line:" + strings.TrimPrefix(style.Line, "#")
	}
	if style.Text != "" {
		color += ";text:" + strings.TrimPrefix(style.Text, "#")
	}
	return color
}

// plantumlArrow is how an edge of each kind is drawn: a connection as the
// Pilot's heavy undirected connector, a binding a plain undirected line, a flow
// dashed, a general graph's relationships in the class-diagram notation, every
// other edge a plain arrow.
func plantumlArrow(kind Kind, edge EdgeKind) string {
	if caseNotation(kind) && (edge == EdgeTyping || edge == EdgeReference) {
		return "..>"
	}
	switch edge {
	case EdgeConnection:
		return "-[thickness=3]-"
	case EdgeBinding:
		return "--"
	case EdgeFlow:
		return "-[dashed]->"
	case EdgeSpecialization:
		return "--|>"
	case EdgeTyping:
		return "..|>"
	case EdgeComposition:
		return "*--"
	case EdgeReference:
		return "o--"
	case EdgeContainment:
		return "+--"
	case EdgeImport, EdgeSatisfy, EdgeVerify, EdgeDerive, EdgeRefine, EdgeAllocate:
		return "..>"
	case EdgeAssociation:
		return "--"
	case EdgeInclude:
		return "..>"
	case EdgeAnchor:
		return ".."
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
		if _, unlinked := plantumlUnlinkedPseudostates[pseudostate]; !unlinked || (w.kind != KindState && w.kind != KindAction) {
			if url, ok := w.links.URL(node.Origin); ok {
				fmt.Fprintf(&out, " [[%s]]", url)
			}
		}
		return out.String()
	}
	if node.Kind != "" {
		fmt.Fprintf(&out, " <<%s>>", node.Kind)
	}
	if shape := plantumlShapeStereotype(node); shape != "" && shape != node.Kind {
		fmt.Fprintf(&out, " <<%s>>", shape)
	}
	if url, ok := w.links.URL(node.Origin); ok {
		fmt.Fprintf(&out, " [[%s]]", url)
	}
	switch {
	case w.fills.filled(node):
		out.WriteString(" " + w.fills.fill(node))
		if w.borders {
			out.WriteString(";line:" + strings.TrimPrefix(w.fills.color(node), "#"))
		}
		if node.Style != nil && node.Style.Text != "" {
			out.WriteString(";text:" + strings.TrimPrefix(node.Style.Text, "#"))
		}
	case node.Style != nil && (node.Style.Fill != "" || node.Style.Line != "" || node.Style.Text != ""):
		out.WriteString(" " + plantumlStyleColor(node.Style))
	}
	return out.String()
}

// noteDecoration styles a note node using the rendering's palette and annotation.
func (w *plantumlWriter) noteDecoration(node *Node) string {
	var out strings.Builder
	switch {
	case w.fills.filled(node):
		out.WriteString(" " + w.fills.fill(node))
		if w.borders {
			out.WriteString(";line:" + strings.TrimPrefix(w.fills.color(node), "#"))
		}
		if node.Style != nil && node.Style.Text != "" {
			out.WriteString(";text:" + strings.TrimPrefix(node.Style.Text, "#"))
		}
	case node.Style != nil && (node.Style.Fill != "" || node.Style.Line != "" || node.Style.Text != ""):
		out.WriteString(" " + plantumlStyleColor(node.Style))
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

// PlantUML does not retain links on these pseudostate stereotypes in SVG.
var plantumlUnlinkedPseudostates = map[string]struct{}{
	"start": {}, "fork": {}, "join": {}, "end": {}, "choice": {}, "history": {}, "history*": {},
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

// plantumlLabel is a node's label ready to quote: the keyword line italic at
// the skin's stereotype size, the name line bold under it, every line escaped.
func (w *plantumlWriter) plantumlLabel(node *Node) string {
	var parts []string
	if keyword := w.labels.keyword(node); keyword != "" {
		parts = append(parts, fmt.Sprintf("<size:%d>//%s//</size>", plantumlKeywordFontSize, plantumlText(keyword)))
	}
	parts = append(parts, "**"+plantumlText(w.labels.head(node))+"**")
	for _, line := range w.labels.details(node) {
		parts = append(parts, plantumlText(line))
	}
	return strings.Join(parts, `\n`)
}

func plantumlNoteLinkText(text string) string {
	return strings.ReplaceAll(text, "]", "~]")
}

// plantumlQuote wraps text in double quotes for a PlantUML display name.
func plantumlQuote(text string) string {
	return `"` + text + `"`
}

// plantumlText escapes characters PlantUML could reinterpret as Creole syntax.
// Guillemets in case and mixed relationship labels are escaped separately.
func plantumlText(text string) string {
	runes := []rune(text)
	var out strings.Builder
	for i, c := range runes {
		switch {
		case c == '\n':
			out.WriteString(`\n`)
		case strings.ContainsRune(`"#\<>~`, c),
			strings.ContainsRune("*/_-[]", c) && (i > 0 && runes[i-1] == c || i+1 < len(runes) && runes[i+1] == c):
			fmt.Fprintf(&out, "<U+%04X>", c)
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}
