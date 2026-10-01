package view

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

// Mermaid is the default machine-readable form of a rendering: a Mermaid
// diagram renders where the models are read — in Markdown documentation, in
// the repository's own docs, and in the editors that host the language server —
// without a Graphviz installation, and it has a state-diagram grammar the state
// rendering maps onto directly. Graphviz DOT, written by DOT, is the alternative
// for Graphviz toolchains and for renderings that will carry exact positions;
// PlantUML, written by PlantUML, for PlantUML toolchains.
//
// A graph-shaped rendering is a `flowchart`; a state rendering is a
// `stateDiagram-v2` and a sequence rendering a `sequenceDiagram`. What the
// rendering could not represent is written as comments, so no notice is lost in
// the machine-readable form either.
func (r *Rendering) Mermaid() string {
	return r.MermaidWith(Options{})
}

// MermaidWith is the Mermaid form written with options. It is drawn in the
// stated direction: a flowchart flows that way, and a state diagram states it
// as a `direction` statement. The empty direction keeps each kind's default,
// and a kind no direction applies to ignores it. A palette fills nodes by
// keyword family, and a Cameo style draws its representable colours. A rendering
// some Layout positions draws the nodes the DOT form draws: the placed ones,
// and the unplaced ones too under UnplacedStrip.
func (r *Rendering) MermaidWith(options Options) string {
	r = r.settleUnplaced(options.Unplaced, FormMermaid)
	direction := options.Direction
	var b strings.Builder
	labels := labelsOf(r.Roots, false, nil)
	labels.skin = skinOf(options.Style)
	r.writeMermaidFrontmatter(&b, labels, options)
	if r.View == "" {
		fmt.Fprintf(&b, "%%%% %s rendering", r.Kind)
	} else {
		fmt.Fprintf(&b, "%%%% %s — %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	if options.Style == StyleCameo {
		b.WriteString("%% style cameo\n")
	}
	for _, notice := range r.Notices {
		fmt.Fprintf(&b, "%%%% not represented: %s\n", notice)
	}
	var fills familyFills
	if options.Palette != "" {
		fills = familyFills{palette: options.Palette, tree: r.Kind == KindTree}
		for _, root := range r.Roots {
			fills.collect(root)
		}
	}
	for _, notice := range r.mermaidNotices(options) {
		fmt.Fprintf(&b, "%%%% not represented: %s\n", notice)
	}
	r.writeGeometryComments(&b, "%%")
	if r.Kind != KindState && r.Kind != KindSequence {
		for i, picture := range r.Pictures {
			if _, _, ok := mermaidPictureSource(picture); !ok {
				continue
			}
			fmt.Fprintf(&b, "%%%% layout: picture%d x=%s y=%s w=%s h=%s", i,
				formatCoord(picture.X), formatCoord(picture.Y), formatCoord(picture.Width), formatCoord(picture.Height))
			if picture.Above {
				b.WriteString(" above")
			}
			b.WriteString("\n")
		}
	}
	switch r.Kind {
	case KindState:
		r.writeStateDiagram(&b, direction, labels, options, fills)
	case KindSequence:
		r.writeSequenceDiagram(&b, labels, options)
	default:
		r.writeFlowchart(&b, direction, labels, options, fills)
	}
	return b.String()
}

func (r *Rendering) mermaidNotices(options Options) []string {
	var notices []string
	nodeFonts, edgeFonts, unsupportedStyles, stateStyles := r.mermaidUnrepresentedStyleCounts()
	if nodeFonts > 0 {
		notices = append(notices, fmt.Sprintf("%d node font style(s) not represented", nodeFonts))
	}
	if edgeFonts > 0 {
		notices = append(notices, fmt.Sprintf("%d edge font style(s) not represented", edgeFonts))
	}
	if len(stateStyles) > 0 {
		notices = append(notices, fmt.Sprintf("%d state style(s) not represented: %s",
			len(stateStyles), strings.Join(stateStyles, ", ")))
	}
	if unsupportedStyles > 0 {
		notices = append(notices, fmt.Sprintf("%d style field(s) not represented", unsupportedStyles))
	}
	if options.Style == StyleCameo {
		if options.Palette == "" && r.hasCameoGradient() {
			notices = append(notices, "Cameo gradients are drawn flat")
		}
		if r.Kind != KindSequence {
			notices = append(notices, "Mermaid has no diagram frame with header tab")
		}
	}
	if names := r.namedForks(); len(names) > 0 {
		notices = append(notices, fmt.Sprintf("%d fork/join name(s) (%s); Mermaid's fork bar draws no label",
			len(names), strings.Join(names, ", ")))
	}
	if r.Kind == KindSequence && options.Palette != "" {
		notices = append(notices, fmt.Sprintf("palette %s; Mermaid's sequence diagram cannot fill individual participants (the PlantUML form fills them)", options.Palette))
	}
	switch r.Kind {
	case KindState:
		unsupported := 0
		for _, note := range r.Notes {
			if note.Anchor == "" || !r.hasState(note.Anchor) {
				unsupported++
			}
		}
		if unsupported > 0 {
			notices = append(notices, fmt.Sprintf("%d note(s) not drawn: Mermaid state notes need an anchor to a declared state; free, edge-anchored and pseudostate notes have none", unsupported))
		}
		if count := r.crossBodyFinalTransitions(); count > 0 {
			notices = append(notices, fmt.Sprintf("%d transition(s) into a final cross state bodies; Mermaid keeps those final states explicit", count))
		}
	case KindSequence:
		free := 0
		unsupported := 0
		for _, note := range r.Notes {
			if note.EdgeFrom != "" {
				if !r.hasSequenceParticipant(note.EdgeFrom) || !r.hasSequenceParticipant(note.EdgeTo) {
					unsupported++
				}
			} else if note.Anchor != "" {
				if !r.hasSequenceParticipant(note.Anchor) {
					unsupported++
				}
			} else {
				free++
			}
		}
		if free > 0 {
			notices = append(notices, fmt.Sprintf("%d free note(s) not drawn: Mermaid sequence notes need a participant or message anchor", free))
		}
		if unsupported > 0 {
			notices = append(notices, fmt.Sprintf("%d note(s) not drawn: Mermaid sequence notes need declared participant anchors", unsupported))
		}
	default:
		unsupported := 0
		var undrawnEdgeNotes []string
		for _, note := range r.Notes {
			if note.Anchor == "" && note.EdgeFrom != "" {
				if !r.flowchartNoteEdgeDrawn(note) {
					undrawnEdgeNotes = append(undrawnEdgeNotes, note.EdgeFrom+"->"+note.EdgeTo)
				}
				continue
			}
			if note.Anchor != "" && !r.hasNode(note.Anchor) {
				unsupported++
			}
		}
		if unsupported > 0 {
			notices = append(notices, fmt.Sprintf("%d note anchor(s) not represented: Mermaid flowchart notes need a drawn node", unsupported))
		}
		if len(undrawnEdgeNotes) > 0 {
			notices = append(notices, fmt.Sprintf("%d note(s) not drawn because their edge is not drawn: %s",
				len(undrawnEdgeNotes), strings.Join(undrawnEdgeNotes, ", ")))
		}
	}
	if r.Kind == KindState || r.Kind == KindSequence {
		if len(r.Pictures) > 0 {
			notices = append(notices, pictureNotice(r.Pictures, fmt.Sprintf("a %s diagram draws no picture", r.Kind)))
		}
	} else {
		type pictureGroup struct {
			reason   string
			pictures []Picture
		}
		var undrawn []pictureGroup
		for _, picture := range r.Pictures {
			if _, reason, ok := mermaidPictureSource(picture); !ok {
				found := false
				for i := range undrawn {
					if undrawn[i].reason == reason {
						undrawn[i].pictures = append(undrawn[i].pictures, picture)
						found = true
						break
					}
				}
				if !found {
					undrawn = append(undrawn, pictureGroup{reason: reason, pictures: []Picture{picture}})
				}
			}
		}
		for _, group := range undrawn {
			notices = append(notices, pictureNotice(group.pictures, group.reason))
		}
	}
	if options.Style != "" && options.Style != StylePilot && options.Style != StyleCameo {
		notices = append(notices, styleNotice(options.Style))
	}
	return notices
}

func (r *Rendering) namedForks() []string {
	var names []string
	var walk func(*Node)
	walk = func(node *Node) {
		if isBarKind(node.Kind) && !node.NameSynthesized && node.Name != "" {
			names = append(names, node.Name)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	return names
}

func (r *Rendering) mermaidUnrepresentedStyleCounts() (nodeFonts, edgeFonts, fields int, stateStyles []string) {
	fontStyle := func(style *Style) bool {
		return style.Font != "" || style.FontSize > 0 || style.Bold || style.Italic
	}
	unsupportedColors := func(style *Style) int {
		count := 0
		for _, value := range []string{style.Fill, style.Line, style.Text} {
			if value != "" {
				count++
			}
		}
		return count
	}
	stateFills := familyFills{tree: r.Kind == KindTree}
	var walk func(*Node)
	walk = func(node *Node) {
		if node.Style != nil {
			if r.Kind == KindSequence && fontStyle(node.Style) ||
				node.Style.Font != "" && !mermaidFontFamily(node.Style.Font) {
				nodeFonts++
			}
			if r.Kind == KindState && !stateFills.classable(node) && mermaidStyleCSS(node.Style) != "" {
				name := node.Name
				if name == "" {
					name = node.ID
				}
				stateStyles = append(stateStyles, name)
			}
			if r.Kind == KindSequence {
				fields += unsupportedColors(node.Style)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	for _, edge := range r.Edges {
		if edge.Style == nil {
			continue
		}
		if r.Kind == KindState || r.Kind == KindSequence {
			if fontStyle(edge.Style) {
				edgeFonts++
			}
			fields += unsupportedColors(edge.Style)
		} else if edge.Style.Font != "" && !mermaidFontFamily(edge.Style.Font) {
			edgeFonts++
		}
		if edge.Style.Fill != "" && r.Kind != KindState && r.Kind != KindSequence {
			fields++
		}
	}
	return nodeFonts, edgeFonts, fields, stateStyles
}

func (r *Rendering) hasCameoGradient() bool {
	found := false
	var walk func(*Node)
	walk = func(node *Node) {
		if len(node.Children) == 0 || isMermaidDiamond(node.Kind) {
			fill, _ := cameoFill(node.Kind)
			found = found || strings.Contains(fill, ":")
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	return found
}

func (r *Rendering) hasState(id string) bool {
	var walk func(*Node) bool
	walk = func(node *Node) bool {
		if node.ID == id && node.Kind == "state" {
			return true
		}
		for _, child := range node.Children {
			if walk(child) {
				return true
			}
		}
		return false
	}
	for _, root := range r.Roots {
		if walk(root) {
			return true
		}
	}
	return false
}

func (r *Rendering) hasSequenceParticipant(id string) bool {
	for _, node := range r.Roots {
		if node.ID == id {
			return true
		}
	}
	return false
}

// writeGeometryComments writes the canvas, node placements and edge routes as
// comments opened by prefix (`%%` in Mermaid, `'` in PlantUML), which those
// forms lay out without: the geometry stays readable in the file rather than
// being dropped.
func (r *Rendering) writeGeometryComments(b *strings.Builder, prefix string) {
	if c := r.Canvas; c != nil {
		b.WriteString(prefix + " canvas:")
		if c.Unit != "" {
			b.WriteString(" unit=" + c.Unit)
		}
		if c.HasSize {
			fmt.Fprintf(b, " w=%s h=%s", formatCoord(c.Width), formatCoord(c.Height))
		}
		b.WriteString("\n")
	}
	for _, root := range r.Roots {
		writeLayoutComments(b, prefix, root)
	}
	for _, edge := range r.Edges {
		if len(edge.Route) == 0 {
			continue
		}
		fmt.Fprintf(b, "%s route: %s->%s", prefix, edge.From, edge.To)
		for _, p := range edge.Route {
			fmt.Fprintf(b, " %s,%s", formatCoord(p.X), formatCoord(p.Y))
		}
		b.WriteString("\n")
	}
}

// writeLayoutComments writes the placement of node and of the nodes under it,
// as comments opened by prefix.
func writeLayoutComments(b *strings.Builder, prefix string, node *Node) {
	if g := node.Geometry; g != nil {
		fmt.Fprintf(b, "%s layout: %s x=%s y=%s", prefix, node.ID, formatCoord(g.X), formatCoord(g.Y))
		if g.HasSize {
			fmt.Fprintf(b, " w=%s h=%s", formatCoord(g.Width), formatCoord(g.Height))
		}
		if g.Collapsed {
			b.WriteString(" collapsed")
		}
		b.WriteString("\n")
	}
	for _, child := range node.Children {
		writeLayoutComments(b, prefix, child)
	}
}

// mermaidTitleLine is the height in pixels of one line of a subgraph title.
const mermaidTitleLine = 24

// writeMermaidFrontmatter writes common and grammar-specific theme variables.
func (r *Rendering) writeMermaidFrontmatter(b *strings.Builder, labels labeller, options Options) {
	type themeVariable struct{ name, value string }
	extra := 0
	if r.Kind != KindState && r.Kind != KindSequence && r.Kind != KindTree {
		for _, root := range r.Roots {
			extra = max(extra, clusterTitleExtraLines(root, labels))
		}
	}
	font, size := "Helvetica, Arial, sans-serif", "14px"
	primary, primaryBorder, text, line, clusterBorder, noteFill, noteBorder := "#FFFFFF", "#181818", "#000000", "#181818", "#181818", "#FEFFDD", "#181818"
	if options.Style == StyleCameo {
		font, size = "Arial, Helvetica, sans-serif", "11px"
		primary = strings.SplitN(cameoBlockFill, ":", 2)[0]
		primaryBorder, text, line, clusterBorder, noteFill, noteBorder = cameoBlockLine, cameoTextColor, cameoEdgeColor, cameoFrameColor, cameoNoteFill, cameoLineColor
	}
	fmt.Fprintf(b, "---\nconfig:\n  fontFamily: \"%s\"\n  theme: base\n", font)
	if r.Kind != KindState && r.Kind != KindSequence {
		b.WriteString("  themeCSS: \".edgeLabel rect { opacity: 1 !important; }\"\n")
	}
	b.WriteString("  themeVariables:\n")
	variables := []themeVariable{
		{"fontFamily", font},
		{"fontSize", size},
		{"primaryColor", primary},
		{"secondaryColor", "#FFFFFF"},
		{"tertiaryColor", "#FFFFFF"},
		{"background", "#FFFFFF"},
		{"primaryBorderColor", primaryBorder},
		{"primaryTextColor", text},
		{"lineColor", line},
		{"textColor", text},
		{"noteBkgColor", noteFill},
		{"noteBorderColor", noteBorder},
		{"noteTextColor", text},
	}
	switch {
	case r.Kind == KindState:
		variables = append(variables,
			themeVariable{"stateBkg", "#FFFFFF"},
			themeVariable{"stateBorder", "#181818"},
			themeVariable{"stateLabelColor", "#000000"},
			themeVariable{"compositeBackground", "#FFFFFF"},
			themeVariable{"compositeBorder", "#181818"},
			themeVariable{"compositeTitleBackground", "#FFFFFF"},
			themeVariable{"compositeTitleBorder", "#181818"},
			themeVariable{"transitionColor", line},
			themeVariable{"transitionLabelColor", text},
			themeVariable{"labelBackgroundColor", "#FFFFFF"},
			themeVariable{"specialStateColor", "#181818"},
		)
	case r.Kind == KindSequence:
		variables = append(variables,
			themeVariable{"actorBkg", primary},
			themeVariable{"actorBorder", primaryBorder},
			themeVariable{"actorTextColor", text},
			themeVariable{"actorLineColor", line},
			themeVariable{"signalColor", line},
			themeVariable{"signalTextColor", text},
			themeVariable{"labelBoxBkgColor", primary},
			themeVariable{"labelBoxBorderColor", primaryBorder},
			themeVariable{"labelTextColor", text},
			themeVariable{"loopTextColor", text},
			themeVariable{"activationBorderColor", line},
			themeVariable{"activationBkgColor", primary},
			themeVariable{"sequenceNumberColor", text},
		)
	default:
		variables = append(variables,
			themeVariable{"clusterBkg", "#FFFFFF"},
			themeVariable{"clusterBorder", clusterBorder},
			themeVariable{"edgeLabelBackground", "#FFFFFF"},
		)
	}
	for _, variable := range variables {
		fmt.Fprintf(b, "    %s: \"%s\"\n", variable.name, variable.value)
	}
	if extra > 0 {
		fmt.Fprintf(b, "  flowchart:\n    subGraphTitleMargin:\n      bottom: %d\n", extra*mermaidTitleLine)
	}
	b.WriteString("---\n")
}

// clusterTitleExtraLines is the most lines beyond the first spanned by the
// title of node or of a cluster under it.
func clusterTitleExtraLines(node *Node, labels labeller) int {
	if len(node.Children) == 0 {
		return 0
	}
	extra := len(labels.lines(node)) - 1
	for _, child := range node.Children {
		extra = max(extra, clusterTitleExtraLines(child, labels))
	}
	return extra
}

// writeFlowchart writes the tree, interconnection and action renderings as a
// Mermaid flowchart: a node with children is a subgraph, containment in a tree
// is an edge, and every other edge is the one the rendering holds.
type mermaidFlowWriter struct {
	b              *strings.Builder
	links          int
	linkStyles     map[int]string
	clusterAnchors map[string]string
	cameo          bool
}

func (w *mermaidFlowWriter) edge(from, arrow, label, to string, style *Style, kind EdgeKind) {
	i := w.links
	w.links++
	if label == "" {
		fmt.Fprintf(w.b, "  %s %s %s\n", from, arrow, to)
	} else {
		fmt.Fprintf(w.b, "  %s %s|\"%s\"| %s\n", from, arrow, mermaidText(label), to)
	}
	var props []string
	if style != nil {
		if style.Line != "" {
			props = append(props, "stroke:"+style.Line)
		}
		if style.Text != "" {
			props = append(props, "color:"+style.Text)
		}
		if mermaidFontFamily(style.Font) {
			props = append(props, "font-family:"+style.Font)
		}
		if style.FontSize > 0 {
			props = append(props, fmt.Sprintf("font-size:%gpx", style.FontSize))
		}
		if style.Bold {
			props = append(props, "font-weight:bold")
		}
		if style.Italic {
			props = append(props, "font-style:italic")
		}
	}
	if kind == EdgeConnection && !w.cameo {
		props = append(props, "stroke-width:3px")
	}
	if len(props) > 0 {
		w.linkStyles[i] = strings.Join(props, ",")
	}
}

func (r *Rendering) writeFlowchart(b *strings.Builder, direction Direction, labels labeller, options Options, fills familyFills) {
	w := &mermaidFlowWriter{b: b, linkStyles: map[int]string{}, cameo: options.Style == StyleCameo}
	flowchart := *r
	flowchart.Notes = r.flowchartNotesWithDrawnEdges()
	flow := "TD"
	if r.Kind == KindInterconnection {
		flow = "LR"
	}
	if direction != "" {
		flow = string(direction)
	}
	fmt.Fprintf(b, "flowchart %s\n", flow)
	ports := r.usedPorts()
	portEnds := r.portEnds(ports)
	noteOwners := flowchart.flowchartNoteOwners(ports)
	w.clusterAnchors = r.flowchartClusterAnchors(portEnds, ports, noteOwners)
	if r.blank() && len(r.Pictures) == 0 && len(flowchart.Notes) == 0 {
		fmt.Fprintf(b, "  empty[\"%s\"]\n", mermaidText(r.blankReason(FormMermaid)))
	} else {
		for _, root := range r.Roots {
			if r.Kind == KindTree {
				r.writeTreeNode(w, root, 1, labels, options)
				continue
			}
			flowchart.writeFlowchartNode(w, root, 1, flow, labels, options, ports, noteOwners)
		}
	}
	flowchart.writeNotes(b, noteOwners, "", "  ")
	r.writePictures(b)
	for _, edge := range r.Edges {
		from, to := edge.From, edge.To
		if end := portEnds[edge.FromPort]; end != "" {
			from = end
		}
		if end := portEnds[edge.ToPort]; end != "" {
			to = end
		}
		from = flowchartEndpoint(from, w.clusterAnchors)
		to = flowchartEndpoint(to, w.clusterAnchors)
		w.edge(from, mermaidArrow(edge.Kind), edge.Label, to, edge.Style, edge.Kind)
	}
	for i, note := range flowchart.Notes {
		anchor := note.Anchor
		if anchor == "" {
			anchor = note.EdgeFrom
		}
		if id := noteGroupIDs(flowchart.Notes)[i]; anchor != "" && flowchart.hasNode(anchor) {
			to := flowchart.flowchartNoteEndpoint(i, anchor, noteOwners, w.clusterAnchors, ports)
			w.edge(id, "-.-", "", to, nil, EdgeTransition)
		}
	}
	r.writeMermaidStyles(b, fills, options, false)
	if len(w.clusterAnchors) > 0 {
		b.WriteString("  classDef anchor fill:none,stroke:none\n")
		ids := make([]string, 0, len(w.clusterAnchors))
		for _, id := range w.clusterAnchors {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintf(b, "  class %s anchor\n", strings.Join(ids, ","))
	}
	if len(flowchart.Notes) > 0 {
		if options.Style == StyleCameo {
			fmt.Fprintf(b, "  classDef note fill:%s,stroke:%s\n", cameoNoteFill, cameoLineColor)
		} else {
			b.WriteString("  classDef note fill:#FEFFDD,stroke:#181818\n")
		}
		fmt.Fprintf(b, "  class %s note\n", strings.Join(uniqueNoteGroupIDs(flowchart.Notes), ","))
	}
	for i := 0; i < w.links; i++ {
		if style := w.linkStyles[i]; style != "" {
			fmt.Fprintf(b, "  linkStyle %d %s\n", i, style)
		}
	}
}

func (r *Rendering) flowchartNoteEdgeDrawn(note Note) bool {
	if note.EdgeFrom == "" || note.EdgeTo == "" ||
		!r.hasNode(note.EdgeFrom) || !r.hasNode(note.EdgeTo) {
		return false
	}
	for _, edge := range r.Edges {
		if edge.From == note.EdgeFrom && edge.To == note.EdgeTo {
			return true
		}
	}
	return false
}

func (r *Rendering) flowchartNotesWithDrawnEdges() []Note {
	notes := make([]Note, 0, len(r.Notes))
	for _, note := range r.Notes {
		if note.Anchor == "" && note.EdgeFrom != "" && !r.flowchartNoteEdgeDrawn(note) {
			continue
		}
		notes = append(notes, note)
	}
	return notes
}

func (r *Rendering) writeTreeNode(w *mermaidFlowWriter, node *Node, depth int, labels labeller, options Options) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(w.b, "%s%s%s\n", indent, node.ID, mermaidNodeShape(node, labels, options))
	for _, child := range node.Children {
		r.writeTreeNode(w, child, depth+1, labels, options)
	}
	for _, child := range node.Children {
		w.edge(node.ID, "---", "", child.ID, nil, EdgeBinding)
	}
}

func (r *Rendering) flowchartSubgraph(node *Node, used map[string]map[string]bool) bool {
	return len(node.Children) > 0 || r.hasUsedPorts(node, used)
}

func flowchartEndpoint(id string, anchors map[string]string) string {
	if anchor := anchors[id]; anchor != "" {
		return anchor
	}
	return id
}

func (r *Rendering) flowchartClusterAnchors(portEnds map[string]string, used map[string]map[string]bool, noteOwners map[int]string) map[string]string {
	if r.Kind == KindTree {
		return nil
	}
	subgraphs := map[string]bool{}
	nodes := map[string]*Node{}
	parents := map[string]string{}
	var walk func(*Node, string)
	walk = func(node *Node, parent string) {
		nodes[node.ID] = node
		parents[node.ID] = parent
		if r.flowchartSubgraph(node, used) {
			subgraphs[node.ID] = true
		}
		for _, child := range node.Children {
			walk(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		walk(root, "")
	}
	needed := map[string]bool{}
	mark := func(id string) {
		if subgraphs[id] {
			needed[id] = true
		}
	}
	for _, edge := range r.Edges {
		from, to := edge.From, edge.To
		if end := portEnds[edge.FromPort]; end != "" {
			from = end
		}
		if end := portEnds[edge.ToPort]; end != "" {
			to = end
		}
		mark(from)
		mark(to)
	}
	for i, note := range r.Notes {
		anchor := note.Anchor
		if anchor == "" {
			anchor = note.EdgeFrom
		}
		if anchor != "" && r.hasNode(anchor) {
			if subgraphs[anchor] {
				mark(anchor)
				continue
			}
			owner := noteOwners[noteGroupIndex(r.Notes, i)]
			for parent := parents[anchor]; parent != ""; parent = parents[parent] {
				if parent == owner {
					break
				}
				if subgraphs[parent] {
					mark(parent)
					break
				}
			}
		}
	}
	anchors := make(map[string]string, len(needed))
	for id := range needed {
		anchors[id] = id + "_anchor"
	}
	return anchors
}

func (r *Rendering) flowchartNoteEndpoint(index int, anchor string, owners map[int]string, clusterAnchors map[string]string, used map[string]map[string]bool) string {
	if clusterAnchors[anchor] != "" {
		return clusterAnchors[anchor]
	}
	nodes := map[string]*Node{}
	parents := map[string]string{}
	var walk func(*Node, string)
	walk = func(node *Node, parent string) {
		nodes[node.ID] = node
		parents[node.ID] = parent
		for _, child := range node.Children {
			walk(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		walk(root, "")
	}
	owner := owners[noteGroupIndex(r.Notes, index)]
	for parent := parents[anchor]; parent != ""; parent = parents[parent] {
		if parent == owner {
			break
		}
		if node := nodes[parent]; node != nil && r.flowchartSubgraph(node, used) {
			if endpoint := clusterAnchors[parent]; endpoint != "" {
				return endpoint
			}
		}
	}
	return flowchartEndpoint(anchor, clusterAnchors)
}

func (r *Rendering) flowchartNoteOwners(used map[string]map[string]bool) map[int]string {
	nodes := map[string]*Node{}
	parents := map[string]string{}
	var walk func(*Node, string)
	walk = func(node *Node, parent string) {
		nodes[node.ID] = node
		parents[node.ID] = parent
		for _, child := range node.Children {
			walk(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		walk(root, "")
	}
	within := func(ancestor, descendant string) bool {
		for id := descendant; id != ""; id = parents[id] {
			if id == ancestor {
				return true
			}
		}
		return false
	}
	owners := map[int]string{}
	for i := range r.Notes {
		group := noteGroupIndex(r.Notes, i)
		if group != i {
			continue
		}
		var anchors []string
		for j, note := range r.Notes {
			if noteGroupIndex(r.Notes, j) != group {
				continue
			}
			anchor := note.Anchor
			if anchor == "" {
				anchor = note.EdgeFrom
			}
			if nodes[anchor] != nil {
				anchors = append(anchors, anchor)
			}
		}
		for candidate := ""; len(anchors) > 0; {
			candidate = anchors[0]
			for candidate != "" {
				encloses := true
				for _, anchor := range anchors[1:] {
					encloses = encloses && within(candidate, anchor)
				}
				if encloses {
					if node := nodes[candidate]; node != nil && r.Kind != KindTree && r.flowchartSubgraph(node, used) {
						owners[group] = candidate
					} else if parent := parents[candidate]; parent != "" {
						if node := nodes[parent]; node != nil && r.Kind != KindTree && r.flowchartSubgraph(node, used) {
							owners[group] = parent
						}
					}
					break
				}
				candidate = parents[candidate]
			}
			break
		}
	}
	return owners
}

func (r *Rendering) writeMermaidStyles(b *strings.Builder, fills familyFills, options Options, state bool) {
	type assignment struct{ class, css string }
	var definitions []assignment
	pairs := map[string]string{}
	classIDs := map[string][]string{}
	next := 0
	control := false
	var walk func(*Node)
	walk = func(node *Node) {
		if !state && isBarKind(node.Kind) {
			control = true
		}
		var css string
		if fills.filled(node) {
			fill, border := fills.fill(node), fills.color(node)
			css = "fill:" + fill + ",stroke:" + border
		} else if options.Style == StyleCameo && (len(node.Children) == 0 || isMermaidDiamond(node.Kind)) && !isMermaidControl(node.Kind) {
			fill, border := cameoFill(node.Kind)
			css = "fill:" + strings.SplitN(fill, ":", 2)[0] + ",stroke:" + border
		} else if options.Style == StyleCameo && isMermaidDiamond(node.Kind) {
			fill, border := cameoFill(node.Kind)
			css = "fill:" + strings.SplitN(fill, ":", 2)[0] + ",stroke:" + border
		}
		if css != "" {
			class, ok := pairs[css]
			if !ok {
				pairs[css] = fmt.Sprintf("palette%d", next)
				class = pairs[css]
				next++
				definitions = append(definitions, assignment{class: class, css: css})
			}
			classIDs[class] = append(classIDs[class], node.ID)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	if control && !state {
		b.WriteString("  classDef control fill:#181818,stroke:#181818\n")
	}
	for _, item := range definitions {
		fmt.Fprintf(b, "  classDef %s %s\n", item.class, item.css)
	}
	for _, item := range definitions {
		if ids := classIDs[item.class]; len(ids) > 0 {
			fmt.Fprintf(b, "  class %s %s\n", strings.Join(ids, ","), item.class)
		}
	}
	var writeStyles func(*Node)
	writeStyles = func(node *Node) {
		if node.Style != nil && (!state || fills.classable(node)) {
			if styleCSS := mermaidStyleCSS(node.Style); styleCSS != "" {
				if state {
					fmt.Fprintf(b, "  classDef style_%s %s\n  class %s style_%s\n", node.ID, styleCSS, node.ID, node.ID)
				} else {
					fmt.Fprintf(b, "  style %s %s\n", node.ID, styleCSS)
				}
			}
		}
		for _, child := range node.Children {
			writeStyles(child)
		}
	}
	if control && !state {
		var ids []string
		var find func(*Node)
		find = func(node *Node) {
			if isBarKind(node.Kind) {
				ids = append(ids, node.ID)
			}
			for _, child := range node.Children {
				find(child)
			}
		}
		for _, root := range r.Roots {
			find(root)
		}
		if len(ids) > 0 {
			fmt.Fprintf(b, "  class %s control\n", strings.Join(ids, ","))
		}
	}
	for _, root := range r.Roots {
		writeStyles(root)
	}
}

func isMermaidControlNode(kind string) bool {
	return kind == startKind || kind == "initial" || kind == "fork" || kind == "join"
}

func isMermaidDiamond(kind string) bool {
	return kind == "decision" || kind == "merge" || kind == "choice"
}

func isMermaidControl(kind string) bool {
	return isMermaidControlNode(kind) || kind == "final" || kind == terminateKind ||
		kind == "junction" || isHistoryKind(kind) || isMermaidDiamond(kind)
}

// mermaidStyleCSS is a Style's fields as Mermaid's comma-separated CSS.
func mermaidStyleCSS(style *Style) string {
	if style == nil {
		return ""
	}
	var props []string
	if style.Fill != "" {
		props = append(props, "fill:"+style.Fill)
	}
	if style.Line != "" {
		props = append(props, "stroke:"+style.Line)
	}
	if style.Text != "" {
		props = append(props, "color:"+style.Text)
	}
	if mermaidFontFamily(style.Font) {
		props = append(props, "font-family:"+style.Font)
	}
	if style.FontSize > 0 {
		props = append(props, fmt.Sprintf("font-size:%gpx", style.FontSize))
	}
	if style.Bold {
		props = append(props, "font-weight:bold")
	}
	if style.Italic {
		props = append(props, "font-style:italic")
	}
	return strings.Join(props, ",")
}

func mermaidFontFamily(font string) bool {
	if strings.TrimSpace(font) == "" {
		return false
	}
	for _, r := range font {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// writeFlowchartNode writes a subgraph for a node with children or used ports.
func (r *Rendering) writeFlowchartNode(w *mermaidFlowWriter, node *Node, depth int, flow string, labels labeller, options Options, used map[string]map[string]bool, noteOwners map[int]string) {
	indent := strings.Repeat("  ", depth)
	if !r.flowchartSubgraph(node, used) {
		fmt.Fprintf(w.b, "%s%s%s\n", indent, node.ID, mermaidNodeShape(node, labels, options))
		return
	}
	fmt.Fprintf(w.b, "%ssubgraph %s [%s]\n", indent, node.ID, mermaidNodeLabel(node, labels, options))
	fmt.Fprintf(w.b, "%s  direction %s\n", indent, flow)
	if anchor := w.clusterAnchors[node.ID]; anchor != "" {
		fmt.Fprintf(w.b, "%s  %s[\" \"]\n", indent, anchor)
	}
	if r.hasUsedPorts(node, used) {
		for j, port := range node.Ports {
			if !used[node.ID][port.ID] {
				continue
			}
			fmt.Fprintf(w.b, "%s  %s_p%d[\"%s\"]\n", indent, node.ID, j, mermaidText(port.Name))
		}
	}
	for _, child := range node.Children {
		r.writeFlowchartNode(w, child, depth+1, flow, labels, options, used, noteOwners)
	}
	r.writeNotes(w.b, noteOwners, node.ID, indent+"  ")
	fmt.Fprintf(w.b, "%send\n", indent)
}

func mermaidNodeShape(node *Node, labels labeller, options Options) string {
	switch node.Kind {
	case startKind, "initial":
		return `@{ shape: f-circ, label: "" }`
	case "final", terminateKind:
		return `@{ shape: fr-circ, label: "" }`
	case "junction":
		return `@{ shape: f-circ, label: "" }`
	case "fork", "join":
		name := ""
		if !node.NameSynthesized {
			name = mermaidText(node.Name)
		}
		return `@{ shape: fork, label: "` + name + `" }`
	case "decision", "merge", "choice":
		name := ""
		if !node.NameSynthesized && node.Name != "" {
			name = mermaidNodeLabel(node, labels, options)
		} else {
			name = `" "`
		}
		return "{" + name + "}"
	case shallowHistoryKind:
		return `(("H"))`
	case deepHistoryKind:
		return `(("H*"))`
	}
	rounded := true
	if options.Style == StyleCameo {
		rounded = cameoRounded(node.Kind)
	} else {
		rounded = plantumlShapeStereotype(node) == plantumlUsageStereotype
	}
	if rounded {
		return "(" + mermaidNodeLabel(node, labels, options) + ")"
	}
	return "[" + mermaidNodeLabel(node, labels, options) + "]"
}

func mermaidNodeLabel(node *Node, labels labeller, options Options) string {
	lines := labels.lines(node)
	for _, line := range lines {
		if !mermaidMarkdownSafe(line) {
			return `"` + labels.mermaid(node) + `"`
		}
	}
	head := labels.headLines(node)
	markdown := make([]string, 0, len(lines))
	for _, line := range head {
		markdown = append(markdown, "**"+line+"**")
	}
	next := len(head)
	if labels.keyworded(node) {
		markdown = append(markdown, "*«"+node.Kind+"»*")
		next++
	}
	markdown = append(markdown, lines[next:]...)
	return `"` + "`" + strings.Join(markdown, "\n") + "`" + `"`
}

var mermaidMultiplicity = regexp.MustCompile(`\[[0-9.*]*\]`)

func mermaidMarkdownSafe(line string) bool {
	if strings.ContainsAny(line, "`\"<>\\#&~|") || strings.Contains(line, "](") {
		return false
	}
	multiplicities := mermaidMultiplicity.ReplaceAllString(line, "")
	if strings.Contains(multiplicities, "*") {
		return false
	}
	for i, r := range line {
		if r == '_' {
			before, after := runeAt(line, i, -1), runeAt(line, i, 1)
			if !unicode.IsLetter(before) && !unicode.IsDigit(before) ||
				!unicode.IsLetter(after) && !unicode.IsDigit(after) {
				return false
			}
		}
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "=") {
		return false
	}
	if len(trimmed) >= 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		for i := 1; i < len(trimmed); i++ {
			if trimmed[i] == '.' || trimmed[i] == ')' {
				return false
			}
			if trimmed[i] < '0' || trimmed[i] > '9' {
				break
			}
		}
	}
	return true
}

func runeAt(text string, index, direction int) rune {
	if direction < 0 {
		for i := index - 1; i >= 0; i-- {
			if text[i]&0xc0 != 0x80 {
				r, _ := utf8.DecodeRuneInString(text[i:])
				return r
			}
		}
		return 0
	}
	for i := index + 1; i < len(text); i++ {
		if text[i]&0xc0 != 0x80 {
			r, _ := utf8.DecodeRuneInString(text[i:])
			return r
		}
	}
	return 0
}

func (r *Rendering) usedPorts() map[string]map[string]bool {
	used := map[string]map[string]bool{}
	if r.Kind != KindAction && r.Kind != KindInterconnection {
		return used
	}
	for _, edge := range r.Edges {
		for _, endpoint := range []struct{ owner, port string }{
			{owner: edge.From, port: edge.FromPort},
			{owner: edge.To, port: edge.ToPort},
		} {
			owner, port := endpoint.owner, endpoint.port
			if port == "" {
				continue
			}
			if used[owner] == nil {
				used[owner] = map[string]bool{}
			}
			used[owner][port] = true
		}
	}
	return used
}

func (r *Rendering) hasUsedPorts(node *Node, used map[string]map[string]bool) bool {
	return len(used[node.ID]) > 0
}

func (r *Rendering) portEnds(used map[string]map[string]bool) map[string]string {
	ends := map[string]string{}
	var walk func(*Node)
	walk = func(node *Node) {
		for i, port := range node.Ports {
			if used[node.ID][port.ID] {
				ends[port.ID] = fmt.Sprintf("%s_p%d", node.ID, i)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	return ends
}

func (r *Rendering) writeNotes(b *strings.Builder, owners map[int]string, owner, indent string) {
	for i := range r.Notes {
		if noteGroupIndex(r.Notes, i) != i {
			continue
		}
		if owners[i] != owner {
			continue
		}
		var lines []string
		for j, member := range r.Notes {
			if noteGroupIndex(r.Notes, j) == i {
				for _, line := range strings.Split(member.Text, "\n") {
					lines = append(lines, mermaidText(line))
				}
			}
		}
		fmt.Fprintf(b, "%snote%d@{ shape: notch-rect, label: \"%s\" }\n", indent, i, strings.Join(lines, "<br>"))
	}
}

func noteGroupIDs(notes []Note) []string {
	ids := make([]string, len(notes))
	for i := range notes {
		ids[i] = fmt.Sprintf("note%d", noteGroupIndex(notes, i))
	}
	return ids
}

func uniqueNoteGroupIDs(notes []Note) []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range noteGroupIDs(notes) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func noteGroupIndex(notes []Note, index int) int {
	origin := notes[index].Origin
	if !origin.Located() {
		return index
	}
	for i := 0; i < index; i++ {
		if notes[i].Origin.Located() && notes[i].Origin == origin {
			return i
		}
	}
	return index
}

func (r *Rendering) hasNode(id string) bool {
	var walk func(*Node) bool
	walk = func(node *Node) bool {
		if node.ID == id {
			return true
		}
		for _, child := range node.Children {
			if walk(child) {
				return true
			}
		}
		return false
	}
	for _, root := range r.Roots {
		if walk(root) {
			return true
		}
	}
	return false
}

func (r *Rendering) writePictures(b *strings.Builder) {
	for i, picture := range r.Pictures {
		src, _, ok := mermaidPictureSource(picture)
		if !ok {
			continue
		}
		fmt.Fprintf(b, "  picture%d@{ img: \"%s\", label: \"%s\", w: %s, h: %s }\n",
			i, mermaidText(src), mermaidText(picture.Alt), formatCoord(picture.Width), formatCoord(picture.Height))
	}
}

func mermaidPictureSource(picture Picture) (string, string, bool) {
	src := picture.Location
	if strings.Contains(src, "://") || strings.HasPrefix(src, "data:") {
		if strings.ContainsAny(src, "\"\r\n") {
			return "", "the source cannot be represented", false
		}
		return src, "", true
	}
	path := picture.Path()
	info, err := os.Stat(path)
	if err != nil {
		return "", "the file does not read", false
	}
	if !info.Mode().IsRegular() {
		return "", "the file is not a regular image", false
	}
	src = filepath.ToSlash(path)
	if strings.ContainsAny(src, "\"\r\n") {
		return "", "the source cannot be represented", false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- Mermaid pictures are intentionally loaded from their declared path.
	if err != nil {
		return "", "the file does not read", false
	}
	if imagefile.ContentType(data) == "" {
		return "", "the image type is not supported", false
	}
	return src, "", true
}

// writeStateDiagram writes a state rendering as a Mermaid state diagram: bodies
// are composite states, entry transitions leave the `[*]` marker of their body.
func (r *Rendering) writeStateDiagram(b *strings.Builder, direction Direction, labels labeller, options Options, fills familyFills) {
	b.WriteString("stateDiagram-v2\n")
	if direction != "" {
		fmt.Fprintf(b, "  direction %s\n", direction)
	}
	if r.blank() {
		// A state diagram takes a note only attached to a state, so the reason
		// is a state of its own.
		fmt.Fprintf(b, "  state \"%s\" as empty\n", mermaidText(r.blankReason(FormMermaid)))
		return
	}
	starts := map[string][]Edge{}
	parent := map[string]string{}
	kinds := map[string]string{}
	var index func(*Node, string)
	index = func(node *Node, owner string) {
		parent[node.ID] = owner
		kinds[node.ID] = node.Kind
		for _, child := range node.Children {
			index(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		collectStarts(root, starts)
		index(root, "")
	}
	for _, edge := range r.Edges {
		if _, ok := starts[edge.From]; ok {
			starts[edge.From] = append(starts[edge.From], edge)
		}
	}
	stateFinalEdges := map[string][]Edge{}
	crossBodyFinals := map[string]bool{}
	for _, edge := range r.Edges {
		if kinds[edge.To] != "final" {
			continue
		}
		if parent[edge.From] == parent[edge.To] {
			stateFinalEdges[parent[edge.To]] = append(stateFinalEdges[parent[edge.To]], edge)
		} else {
			crossBodyFinals[edge.To] = true
		}
	}
	convertedFinals := map[string]bool{}
	for _, edges := range stateFinalEdges {
		for _, edge := range edges {
			if !crossBodyFinals[edge.To] {
				convertedFinals[edge.To] = true
			}
		}
	}
	for _, root := range r.Roots {
		if convertedFinals[root.ID] {
			continue
		}
		r.writeStateNode(b, root, 1, starts, stateFinalEdges, convertedFinals, parent, kinds, labels)
	}
	for _, edge := range stateFinalEdges[""] {
		from := edge.From
		if kinds[from] == startKind {
			from = "[*]"
		}
		writeStateEdge(b, from, "[*]", edge.Label, 1)
	}
	for _, edge := range r.Edges {
		if _, ok := starts[edge.From]; ok {
			continue
		}
		if kinds[edge.To] == "final" && parent[edge.From] == parent[edge.To] {
			continue
		}
		writeStateEdge(b, edge.From, edge.To, edge.Label, 1)
	}
	r.writeMermaidStyles(b, fills, options, true)
}

// collectStarts records the start node of each body under node, to gather the
// edges leaving it.
func collectStarts(node *Node, starts map[string][]Edge) {
	if node.Kind == startKind {
		starts[node.ID] = nil
	}
	for _, child := range node.Children {
		collectStarts(child, starts)
	}
}

// writeStateEdge writes one transition, with its label when it carries one.
func writeStateEdge(b *strings.Builder, from, to, label string, depth int) {
	indent := strings.Repeat("  ", depth)
	if label == "" {
		fmt.Fprintf(b, "%s%s --> %s\n", indent, from, to)
		return
	}
	fmt.Fprintf(b, "%s%s --> %s : %s\n", indent, from, to, mermaidTransitionText(label))
}

// writeSequenceDiagram writes a sequence rendering as a Mermaid sequence
// diagram: one participant per lifeline, declared before the messages, then the
// messages in the order the rendering settled on.
func (r *Rendering) writeSequenceDiagram(b *strings.Builder, labels labeller, options Options) {
	_ = options
	b.WriteString("sequenceDiagram\n")
	if r.blank() {
		// A sequence diagram carries no free text, so the reason is a
		// participant of its own.
		fmt.Fprintf(b, "  participant empty as %s\n", mermaidText(r.blankReason(FormMermaid)))
		return
	}
	for _, node := range r.Roots {
		fmt.Fprintf(b, "  participant %s as %s\n", node.ID, labels.mermaid(node))
	}
	r.writeSequenceNotes(b)
	writtenNotes := map[int]bool{}
	for _, edge := range r.Edges {
		// The colon is part of the message syntax; only the text after it is left
		// off when the message carries none.
		if edge.Label == "" {
			fmt.Fprintf(b, "  %s->>%s:\n", edge.From, edge.To)
		} else {
			fmt.Fprintf(b, "  %s->>%s: %s\n", edge.From, edge.To, mermaidText(edge.Label))
		}
		for i, note := range r.Notes {
			if note.EdgeFrom == "" || writtenNotes[i] {
				continue
			}
			if (edge.From == note.EdgeFrom && edge.To == note.EdgeTo) ||
				(edge.From == note.EdgeTo && edge.To == note.EdgeFrom) {
				r.writeSequenceNote(b, note)
				writtenNotes[i] = true
			}
		}
	}
	for i, note := range r.Notes {
		if note.EdgeFrom != "" && !writtenNotes[i] {
			r.writeSequenceNote(b, note)
		}
	}
}

// writeStateNode writes one state and its substates. A body's start is the `[*]`
// marker inside that state, so its edges are written there after the substates.
func (r *Rendering) writeStateNode(b *strings.Builder, node *Node, depth int, starts, finalEdges map[string][]Edge, convertedFinals map[string]bool, parents, kinds map[string]string, labels labeller) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 {
		r.writeStateDeclaration(b, node, indent, labels)
		r.writeStateNotes(b, node.ID, depth)
		return
	}
	fmt.Fprintf(b, "%sstate \"%s\" as %s {\n", indent, labels.mermaid(node), node.ID)
	for _, child := range node.Children {
		if child.Kind != startKind && !convertedFinals[child.ID] {
			r.writeStateNode(b, child, depth+1, starts, finalEdges, convertedFinals, parents, kinds, labels)
		}
	}
	for _, child := range node.Children {
		for _, edge := range starts[child.ID] {
			if kinds[edge.To] == "final" && parents[edge.From] == parents[edge.To] {
				continue
			}
			writeStateEdge(b, "[*]", edge.To, edge.Label, depth+1)
		}
	}
	for _, edge := range finalEdges[node.ID] {
		from := edge.From
		if _, ok := starts[from]; ok {
			from = "[*]"
		}
		writeStateEdge(b, from, "[*]", edge.Label, depth+1)
	}
	fmt.Fprintf(b, "%s}\n", indent)
	r.writeStateNotes(b, node.ID, depth)
}

func (r *Rendering) crossBodyFinalTransitions() int {
	parent := map[string]string{}
	kinds := map[string]string{}
	var walk func(*Node, string)
	walk = func(node *Node, owner string) {
		parent[node.ID], kinds[node.ID] = owner, node.Kind
		for _, child := range node.Children {
			walk(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		walk(root, "")
	}
	count := 0
	for _, edge := range r.Edges {
		if kinds[edge.To] == "final" && parent[edge.From] != parent[edge.To] {
			count++
		}
	}
	return count
}

func (r *Rendering) writeStateDeclaration(b *strings.Builder, node *Node, indent string, labels labeller) {
	switch node.Kind {
	case "fork", "join", "decision", "merge", "choice", "junction":
		kind := node.Kind
		if kind == "decision" || kind == "merge" || kind == "junction" {
			kind = "choice"
		}
		fmt.Fprintf(b, "%sstate %s <<%s>>\n", indent, node.ID, kind)
	case shallowHistoryKind, deepHistoryKind:
		label := "H"
		if node.Kind == deepHistoryKind {
			label = "H*"
		}
		fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, label, node.ID)
	default:
		fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, labels.mermaid(node), node.ID)
	}
}

func (r *Rendering) writeStateNotes(b *strings.Builder, anchor string, depth int) {
	for _, note := range r.Notes {
		if note.Anchor != anchor || !r.hasState(anchor) {
			continue
		}
		fmt.Fprintf(b, "%snote right of %s\n", strings.Repeat("  ", depth+1), anchor)
		for _, line := range strings.Split(note.Text, "\n") {
			fmt.Fprintf(b, "%s  %s\n", strings.Repeat("  ", depth+1), mermaidText(line))
		}
		fmt.Fprintf(b, "%send note\n", strings.Repeat("  ", depth+1))
	}
}

func (r *Rendering) writeSequenceNotes(b *strings.Builder) {
	for _, note := range r.Notes {
		if note.Anchor != "" && note.EdgeFrom == "" && r.hasSequenceParticipant(note.Anchor) {
			r.writeSequenceNote(b, note)
		}
	}
}

func (r *Rendering) writeSequenceNote(b *strings.Builder, note Note) {
	lines := strings.Split(note.Text, "\n")
	for i, line := range lines {
		lines[i] = mermaidText(line)
	}
	text := strings.Join(lines, "<br>")
	if note.EdgeFrom != "" {
		if !r.hasSequenceParticipant(note.EdgeFrom) || !r.hasSequenceParticipant(note.EdgeTo) {
			return
		}
		fmt.Fprintf(b, "  Note over %s,%s: %s\n", note.EdgeFrom, note.EdgeTo, text)
		return
	}
	if note.Anchor != "" {
		fmt.Fprintf(b, "  Note over %s: %s\n", note.Anchor, text)
	}
}

var mermaidImagePattern = regexp.MustCompile(`^\s*picture[0-9]+@\{[^}\r\n]*img:\s*"([^"\r\n]*)"`)

// InlineMermaidImages embeds each readable local flowchart image in source.
// base is the working directory used to resolve relative image paths.
func InlineMermaidImages(source, base string) string {
	lines := strings.Split(source, "\n")
	size := len(source)
	var notices []string
	drop := func(index int, location, reason string) {
		old := lines[index]
		lines[index] = ""
		notice := fmt.Sprintf("%%%% not represented: picture %s not drawn; %s", location, reason)
		notices = append(notices, notice)
		size += len(notice) + 1 - len(old)
	}
	for i, line := range lines {
		match := mermaidImagePattern.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		location := line[match[2]:match[3]]
		if strings.Contains(location, "://") || strings.HasPrefix(location, "data:") {
			continue
		}
		path := location
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, filepath.FromSlash(path))
		}
		data, err := os.ReadFile(path) // #nosec G304 -- Mermaid pictures are intentionally loaded from their declared path.
		if err != nil {
			drop(i, location, "the file does not read")
			continue
		}
		contentType := imagefile.ContentType(data)
		if contentType == "" {
			drop(i, location, "the file is not a supported image")
			continue
		}
		uri := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
		replaced := line[:match[2]] + uri + line[match[3]:]
		delta := len(replaced) - len(line)
		if size+delta+1 > MermaidTextCeiling {
			drop(i, location, "inlining would exceed Mermaid's text ceiling")
			continue
		}
		lines[i] = replaced
		size += delta
	}
	if len(notices) == 0 {
		return strings.Join(lines, "\n")
	}
	headerEnd := 0
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start, headerEnd = i+1, i+1
				break
			}
		}
	}
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			headerEnd = i + 1
			continue
		}
		break
	}
	lines = append(lines[:headerEnd], append(notices, lines[headerEnd:]...)...)
	return strings.Join(lines, "\n")
}

// mermaid is a node's label ready to embed: its lines escaped and joined with
// `<br>`, which flowcharts, state diagrams and sequence diagrams all break at.
func (l labeller) mermaid(node *Node) string {
	lines := l.lines(node)
	for i, line := range lines {
		lines[i] = mermaidText(line)
	}
	return strings.Join(lines, "<br>")
}

// mermaidArrow is how an edge of each kind is drawn in a flowchart.
func mermaidArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection, EdgeBinding:
		return "==="
	case EdgeFlow:
		return "-.->"
	}
	return "-->"
}

// mermaidText escapes what a Mermaid label may not carry literally. A semicolon
// ends a statement, which an unquoted label — a sequence participant or a
// message — would be cut short by.
func mermaidText(text string) string {
	replacer := strings.NewReplacer("#", "#35;", "\"", "#quot;", "\n", " ", "<", "#lt;", ">", "#gt;", ";", "#59;")
	return replacer.Replace(text)
}

// MermaidTextCeiling and MermaidEdgeCeiling bound the maxTextSize and maxEdges
// a chart is ever drawn under: twenty times Mermaid's defaults, so a large
// model's figures draw while no chart asks a browser for unbounded work.
const (
	MermaidTextCeiling = 1_000_000
	MermaidEdgeCeiling = 10_000
)

// MermaidSize is the maxTextSize and maxEdges a chart's source is drawn under:
// one past its length and one past the edges it declares.
func MermaidSize(source string) (textSize, edges int) {
	return len(source) + 1, mermaidEdges(source) + 1
}

// mermaidEdges counts the lines that draw an arrow outside a quoted label; a
// comment declares none.
func mermaidEdges(source string) int {
	edges := 0
	lines := strings.Split(source, "\n")
	frontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	inFrontmatter, inQuote := frontmatter, false
	frontmatterStarted := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if inFrontmatter {
			if line == "---" {
				if frontmatterStarted {
					inFrontmatter = false
				} else {
					frontmatterStarted = true
				}
			}
			continue
		}
		if strings.HasPrefix(line, "%%") {
			continue
		}
		statement := unquotedState(line, &inQuote)
		if declaresEdge(statement) {
			edges++
		}
	}
	return edges
}

// declaresEdge reports whether a statement without its labels draws an arrow:
// a flowchart's or state diagram's between spaces or before its label's bar, or
// a sequence message's.
func declaresEdge(statement string) bool {
	if strings.Contains(statement, "->>") {
		return true
	}
	for _, arrow := range []string{"-->", "---", "-.->", "-.-", "==="} {
		if strings.Contains(statement, " "+arrow+" ") || strings.Contains(statement, " "+arrow+"|") {
			return true
		}
	}
	return false
}

func unquotedState(statement string, inQuote *bool) string {
	var b strings.Builder
	for _, r := range statement {
		if r == '"' {
			*inQuote = !*inQuote
		} else if !*inQuote {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MermaidFits reports whether a chart's source is drawn under the ceilings.
func MermaidFits(source string) bool {
	textSize, edges := MermaidSize(source)
	return textSize <= MermaidTextCeiling && edges <= MermaidEdgeCeiling
}

// MermaidLimits is the maxTextSize and maxEdges every one of the sources fits
// under, which Mermaid's defaults refuse a large chart by, never past the ceilings.
func MermaidLimits(sources ...string) (textSize, edges int) {
	for _, source := range sources {
		t, e := MermaidSize(source)
		textSize = max(textSize, min(t, MermaidTextCeiling))
		edges = max(edges, min(e, MermaidEdgeCeiling))
	}
	return textSize, edges
}

// mermaidTransitionText escapes a state transition's label, which follows an
// unquoted colon: a state diagram reads `::` in it as the class marker, so a
// qualified name in a trigger or guard is written with its colons as entities.
func mermaidTransitionText(text string) string {
	return strings.ReplaceAll(mermaidText(text), ":", "#58;")
}
