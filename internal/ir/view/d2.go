package view

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// D2 is the D2 form of a graph-shaped or sequence rendering, in black and
// white with each kind's default direction.
func (r *Rendering) D2() (string, error) {
	return r.D2With(Options{})
}

// D2With is the rendering as a D2 diagram, drawn as the DOT and PlantUML
// forms draw: a tree as flat nodes joined by containment lines, an
// interconnection as nested containers whose drawn ports are pins inside
// them, a state or action graph as nested containers with its control nodes
// as pseudostate glyphs, a requirement, definition or package graph as nodes
// joined in the class-diagram notation, and a sequence as D2's sequence
// diagram. Direction,
// Palette, Ports and Unplaced apply; a Style does not, D2 drawing in one
// look, and asking for one is noticed. Positions and routes are kept as
// comments, D2 laying the diagram out itself.
func (r *Rendering) D2With(options Options) (string, error) {
	if !r.Kind.SupportsForm(FormD2) {
		return "", &WrongFormError{Form: FormD2, Kind: r.Kind, View: r.View}
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
	r = r.settleUnplaced(options.Unplaced, FormD2)
	w := &d2Writer{
		fills:   familyFills{palette: options.Palette, tree: r.Kind == KindTree},
		labels:  labelsOf(r.Roots, false, nil),
		ports:   r.portView(options.Ports),
		nested:  r.Kind != KindTree,
		glyphs:  r.Kind == KindState || r.Kind == KindAction,
		links:   options.Links,
		paths:   map[string]string{},
		classes: map[string]bool{},
	}
	if r.Kind == KindAction {
		w.used = r.usedPorts(w.ports)
	}
	for _, root := range r.Roots {
		w.fills.collect(root)
	}
	notices := r.d2Notices(options, w)
	switch r.Kind {
	case KindTree:
		w.writeTree(r)
	case KindInterconnection, KindState, KindAction, KindRequirement, KindDefinition, KindPackage:
		w.writeGraph(r)
	case KindSequence:
		w.writeSequence(r)
	}
	var b strings.Builder
	if r.View == "" {
		fmt.Fprintf(&b, "# %s rendering", r.Kind)
	} else {
		fmt.Fprintf(&b, "# %s — %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	for _, notice := range slices.Concat(r.Notices, notices) {
		fmt.Fprintf(&b, "# not represented: %s\n", notice)
	}
	if r.Kind.SupportsDirection() && options.Direction != "" {
		fmt.Fprintf(&b, "direction: %s\n", d2Direction(options.Direction))
	}
	w.writeClasses(&b)
	r.writeGeometryComments(&b, "#")
	b.WriteString(w.body.String())
	return b.String(), nil
}

// d2Writer holds what one rendering's D2 form needs across nodes and edges.
type d2Writer struct {
	body    strings.Builder
	fills   familyFills                // the palette fills, by keyword family
	labels  labeller                   // the node labels, headed relative to the roots' namespace
	ports   portView                   // the ports drawn of each node, and how they are named
	nested  bool                       // whether children are nested containers rather than nodes joined by lines
	glyphs  bool                       // whether control nodes are drawn as pseudostate glyphs
	links   Links                      // the source link each node and edge carries; zero links none
	used    map[string]map[string]bool // the action pins an edge ends at
	paths   map[string]string          // each node's and drawn pin's path from the root scope
	classes map[string]bool            // the classes the body uses
}

// d2Notices are what the D2 form leaves out of a rendering written with options.
func (r *Rendering) d2Notices(options Options, w *d2Writer) []string {
	var notices []string
	if placed, routed := r.countGeometry(); placed+routed > 0 || r.Canvas != nil {
		notices = append(notices, fmt.Sprintf("%d positioned node(s) and %d route(s) kept as comments; D2 pins no position, the dot form does", placed, routed))
	}
	if options.Style != "" && options.Style != StylePilot {
		notices = append(notices, styleNotice(options.Style))
	}
	if r.Kind == KindAction {
		if pins := r.undrawnPins(func(node *Node, port Port) bool { return w.used[node.ID][port.ID] }); len(pins) > 0 {
			notices = append(notices, fmt.Sprintf("%d pin(s) not drawn (%s); D2 draws the pins an edge ends at",
				len(pins), strings.Join(pins, ", ")))
		}
	}
	if w.glyphs {
		if names := r.namedForks(); len(names) > 0 {
			notices = append(notices, fmt.Sprintf("%d fork/join name(s) (%s); D2's fork bar draws no label",
				len(names), strings.Join(names, ", ")))
		}
	}
	if nodes, edges := r.countFonts(); nodes+edges > 0 {
		notices = append(notices, fmt.Sprintf("%d node(s) and %d edge(s) name a font; D2 draws no font family, the dot form does", nodes, edges))
	}
	if len(r.Notes) > 0 {
		notices = append(notices, fmt.Sprintf("%d note(s); the dot form draws notes", len(r.Notes)))
	}
	if len(r.Pictures) > 0 {
		notices = append(notices, pictureNotice(r.Pictures, "the dot form draws pictures"))
	}
	return notices
}

// countFonts is how many nodes and edges of the rendering name a font family.
func (r *Rendering) countFonts() (nodes, edges int) {
	var walk func(node *Node)
	walk = func(node *Node) {
		if node.Style != nil && node.Style.Font != "" {
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
		if edge.Style != nil && edge.Style.Font != "" {
			edges++
		}
	}
	return nodes, edges
}

// d2Direction is D2's name for a direction: it draws all four.
func d2Direction(direction Direction) string {
	switch direction {
	case DirectionBottomTop:
		return "up"
	case DirectionLeftRight:
		return "right"
	case DirectionRightLeft:
		return "left"
	}
	return "down"
}

// d2Element is the look every element shares: white on black, in the
// Pilot's hairline.
const d2Element = `fill: white; stroke: "#181818"; stroke-width: 1; font-color: black`

// d2Arrow is the look every edge shares.
const d2Arrow = `stroke: "#181818"; font-size: 13; font-color: black`

// d2Classes are the classes the body marks nodes and edges with, in the order
// they are declared: the shapes by element kind, the pseudostate glyphs, and
// the edges by their kind.
var d2Classes = []struct{ name, body string }{
	{"definition", "style: { " + d2Element + "; font-size: 14 }"},
	{"usage", "style: { " + d2Element + "; font-size: 14; border-radius: 8 }"},
	{"package", "style: { " + d2Element + "; font-size: 14; stroke-width: 2 }"},
	{"region", "style: { " + d2Element + "; font-size: 14; stroke-dash: 3 }"},
	{"pin", "style: { " + d2Element + "; font-size: 10 }"},
	{"initial", "shape: oval; width: 18; height: 18; style: { fill: black; stroke: black }"},
	{"final", "shape: oval; width: 22; height: 22; style: { fill: black; stroke: black; double-border: true }"},
	{"terminate", "shape: text; style: { font-size: 24; bold: true; font-color: black }"},
	{"bar", "shape: rectangle; width: 80; height: 8; style: { fill: black; stroke: black }"},
	{"choice", "shape: diamond; style: { " + d2Element + "; font-size: 14 }"},
	{"history", "shape: oval; width: 26; height: 26; style: { " + d2Element + "; font-size: 14 }"},
	{"edge", "style: { " + d2Arrow + "; stroke-width: 1 }"},
	{"connection", "style: { " + d2Arrow + "; stroke-width: 3 }"},
	{"flow", "style: { " + d2Arrow + "; stroke-width: 1; stroke-dash: 3 }"},
	{"specialization", "target-arrowhead: { shape: triangle; style: { filled: false } }; style: { " + d2Arrow + "; stroke-width: 1 }"},
	{"typing", "target-arrowhead: { shape: triangle; style: { filled: false } }; style: { " + d2Arrow + "; stroke-width: 1; stroke-dash: 3 }"},
	{"composition", "source-arrowhead: { shape: diamond; style: { filled: true } }; style: { " + d2Arrow + "; stroke-width: 1 }"},
	{"reference", "source-arrowhead: { shape: diamond; style: { filled: false } }; style: { " + d2Arrow + "; stroke-width: 1 }"},
	{"containment", "source-arrowhead: { shape: circle; style: { filled: false } }; style: { " + d2Arrow + "; stroke-width: 1 }"},
	{"dependency", "style: { " + d2Arrow + "; stroke-width: 1; stroke-dash: 3 }"},
}

// writeClasses declares the classes the body uses, so the file stands alone.
func (w *d2Writer) writeClasses(b *strings.Builder) {
	if len(w.classes) == 0 {
		return
	}
	b.WriteString("classes: {\n")
	for _, class := range d2Classes {
		if w.classes[class.name] {
			fmt.Fprintf(b, "  %s: { %s }\n", class.name, class.body)
		}
	}
	b.WriteString("}\n")
}

// writeTree writes a tree rendering: every node at the root scope, each
// joined to its children by a containment line, then the edges.
func (w *d2Writer) writeTree(r *Rendering) {
	if r.blank() {
		w.writeEmpty(r)
		return
	}
	var declare func(node *Node)
	declare = func(node *Node) {
		w.writeNode(node, "", "")
		for _, child := range node.Children {
			declare(child)
		}
	}
	for _, root := range r.Roots {
		declare(root)
	}
	var contain func(node *Node)
	contain = func(node *Node) {
		for _, child := range node.Children {
			w.writeArrow("", w.paths[node.ID], w.paths[child.ID], "--", "", "edge", nil, Origin{})
			contain(child)
		}
	}
	for _, root := range r.Roots {
		contain(root)
	}
	for _, edge := range r.Edges {
		w.writeEdge("", edge)
	}
}

// writeGraph writes an interconnection, state or action rendering: the roots
// as containers holding their pins and children, then the edges between them
// by path.
func (w *d2Writer) writeGraph(r *Rendering) {
	if r.blank() {
		w.writeEmpty(r)
		return
	}
	for _, root := range r.Roots {
		w.writeNode(root, "", "")
	}
	for _, edge := range r.Edges {
		w.writeEdge("", edge)
	}
}

// writeSequence writes a sequence rendering as D2's sequence diagram: one
// actor per lifeline, declared before the messages, then the messages in the
// order the rendering settled on.
func (w *d2Writer) writeSequence(r *Rendering) {
	w.body.WriteString("sequence: \"\" {\n  shape: sequence_diagram\n")
	if r.blank() {
		fmt.Fprintf(&w.body, "  empty: %s\n", d2Quote(r.blankReason(FormD2)))
	} else {
		for _, node := range r.Roots {
			w.writeNode(node, "", "  ")
		}
		for _, edge := range r.Edges {
			w.writeEdge("  ", edge)
		}
	}
	w.body.WriteString("}\n")
}

// writeEmpty writes the one node a blank rendering shows: why it is blank.
func (w *d2Writer) writeEmpty(r *Rendering) {
	fmt.Fprintf(&w.body, "empty: %s\n", d2Quote(r.blankReason(FormD2)))
}

// writeNode writes one node under scope: its label, class, source link and
// style, then the pins it draws and, when children nest, its children, each
// by path. Pins are not linked: a port's link is its owner's.
func (w *d2Writer) writeNode(node *Node, scope, indent string) {
	key := d2Key(node.ID)
	path := d2Path(scope, key)
	w.paths[node.ID] = path
	class := w.class(node)
	attrs := []string{"class: " + class}
	if link := w.link(node.Origin); link != "" {
		attrs = append(attrs, link)
	}
	if style := w.nodeStyle(node); style != "" {
		attrs = append(attrs, "style: { "+style+" }")
	}
	pins := w.pins(node)
	var children []*Node
	if w.nested {
		children = node.Children
	}
	label := d2Quote(w.label(node, class))
	if len(pins) == 0 && len(children) == 0 {
		fmt.Fprintf(&w.body, "%s%s: %s { %s }\n", indent, key, label, strings.Join(attrs, "; "))
		return
	}
	fmt.Fprintf(&w.body, "%s%s: %s {\n", indent, key, label)
	for _, attr := range attrs {
		fmt.Fprintf(&w.body, "%s  %s\n", indent, attr)
	}
	for _, pin := range pins {
		pinKey := d2Key(pin.ID)
		w.paths[pin.ID] = d2Path(path, pinKey)
		w.classes["pin"] = true
		fmt.Fprintf(&w.body, "%s  %s: %s { class: pin }\n", indent, pinKey, d2Quote(w.pinLabel(pin)))
	}
	for _, child := range children {
		w.writeNode(child, path, indent+"  ")
	}
	fmt.Fprintf(&w.body, "%s}\n", indent)
}

// pins are the ports drawn inside a node: an interconnection's under the
// ports display, an action's the ones an edge ends at.
func (w *d2Writer) pins(node *Node) []Port {
	switch {
	case w.ports.interconnection:
		return w.ports.of(node)
	case w.ports.action:
		var pins []Port
		for _, port := range node.Ports {
			if w.used[node.ID][port.ID] {
				pins = append(pins, port)
			}
		}
		return pins
	}
	return nil
}

// pinLabel is a pin's label: on an interconnection the port's stereotype over
// `name : Type` under the full display and the name alone under the minimal,
// on an action the pin's name.
func (w *d2Writer) pinLabel(port Port) string {
	if !w.ports.interconnection {
		return port.Name
	}
	label := w.ports.pinLabel(port)
	if !w.ports.minimal {
		return "«port»\n" + label
	}
	return label
}

// class is the class a node is drawn with, recorded as used.
func (w *d2Writer) class(node *Node) string {
	class := d2NodeClass(node, w.glyphs)
	w.classes[class] = true
	return class
}

// d2NodeClass is the class a node is drawn with: a pseudostate glyph for a
// control node where glyphs are drawn, else the element's shape by kind.
func d2NodeClass(node *Node, glyphs bool) string {
	if glyphs {
		switch node.Kind {
		case startKind, "initial", "junction":
			return "initial"
		case "final":
			return "final"
		case terminateKind:
			return "terminate"
		case "fork", "join":
			return "bar"
		case "decision", "choice", "merge":
			return "choice"
		case "shallow history", "deep history":
			return "history"
		case "region":
			return "region"
		}
	}
	switch {
	case slices.Contains(strings.Fields(node.Kind), "package"):
		return "package"
	case isDefinitionKind(node.Kind):
		return "definition"
	}
	return "usage"
}

// label is a node's label by its class: a glyph draws its mark or nothing, a
// diamond its name when it has one, and every other node its keyword line,
// its head and its details, one per line.
func (w *d2Writer) label(node *Node, class string) string {
	switch class {
	case "initial", "final", "bar":
		return ""
	case "terminate":
		return "✕"
	case "history":
		if node.Kind == "deep history" {
			return "H*"
		}
		return "H"
	case "choice":
		if shown(node) == "" {
			return ""
		}
	}
	return strings.Join(w.labels.lines(node), "\n")
}

// nodeStyle is a node's own style: its palette fill with the border in the
// family colour, else the colours its Style states, then the Style's size and
// weight. A font family is not drawn, D2 naming none.
func (w *d2Writer) nodeStyle(node *Node) string {
	var attrs []string
	switch {
	case w.fills.filled(node):
		attrs = append(attrs, "fill: "+d2Color(w.fills.fill(node)), "stroke: "+d2Color(w.fills.color(node)))
		if node.Style != nil && node.Style.Text != "" {
			attrs = append(attrs, "font-color: "+d2Color(node.Style.Text))
		}
	case node.Style != nil:
		attrs = append(attrs, d2Colors(node.Style, true)...)
	}
	if node.Style != nil {
		attrs = append(attrs, d2Font(node.Style)...)
	}
	return strings.Join(attrs, "; ")
}

// d2Colors are a Style's colours as D2 style attributes; a node takes its
// fill, an edge has none.
func d2Colors(style *Style, fill bool) []string {
	var attrs []string
	if fill && style.Fill != "" {
		attrs = append(attrs, "fill: "+d2Color(style.Fill))
	}
	if style.Line != "" {
		attrs = append(attrs, "stroke: "+d2Color(style.Line))
	}
	if style.Text != "" {
		attrs = append(attrs, "font-color: "+d2Color(style.Text))
	}
	return attrs
}

// The font sizes D2 accepts.
const (
	d2MinFontSize = 8
	d2MaxFontSize = 100
)

// d2Font is a Style's size and weight as D2 style attributes, the size
// held within what D2 accepts.
func d2Font(style *Style) []string {
	var attrs []string
	if style.FontSize > 0 {
		size := min(max(int(math.Round(style.FontSize)), d2MinFontSize), d2MaxFontSize)
		attrs = append(attrs, fmt.Sprintf("font-size: %d", size))
	}
	if style.Bold {
		attrs = append(attrs, "bold: true")
	}
	if style.Italic {
		attrs = append(attrs, "italic: true")
	}
	return attrs
}

// d2Color quotes a colour as `"#RRGGBB"`, a bare `#` opening a comment.
func d2Color(color string) string {
	return `"#` + strings.TrimPrefix(color, "#") + `"`
}

// writeEdge writes one edge as its kind's connector: a connection the
// Pilot's heavy undirected line, a binding a plain one, a flow dashed, a
// general graph's relationships in the class-diagram notation, every other
// edge an arrow; in a sequence diagram every message is an arrow.
func (w *d2Writer) writeEdge(indent string, edge Edge) {
	arrow, class := "->", "edge"
	switch edge.Kind {
	case EdgeConnection:
		arrow, class = "--", "connection"
	case EdgeBinding:
		arrow = "--"
	case EdgeFlow:
		class = "flow"
	case EdgeSpecialization, EdgeTyping:
		class = edge.Kind.String()
	case EdgeComposition, EdgeReference, EdgeContainment:
		// D2 draws a source arrowhead only where the connection has a head there.
		arrow, class = "<-", edge.Kind.String()
	case EdgeImport, EdgeSatisfy, EdgeVerify, EdgeDerive, EdgeRefine, EdgeAllocate:
		class = "dependency"
	}
	w.writeArrow(indent, w.end(edge.From, edge.FromPort), w.end(edge.To, edge.ToPort), arrow, edge.Label, class, edge.Style, edge.Origin)
}

// writeArrow writes one connection statement between two paths, with its
// label when it carries one, its source link and the colours and font of its
// Style.
func (w *d2Writer) writeArrow(indent, from, to, arrow, label, class string, style *Style, origin Origin) {
	w.classes[class] = true
	attrs := []string{"class: " + class}
	if link := w.link(origin); link != "" {
		attrs = append(attrs, link)
	}
	if style != nil {
		if own := slices.Concat(d2Colors(style, false), d2Font(style)); len(own) > 0 {
			attrs = append(attrs, "style: { "+strings.Join(own, "; ")+" }")
		}
	}
	if label == "" {
		fmt.Fprintf(&w.body, "%s%s %s %s: { %s }\n", indent, from, arrow, to, strings.Join(attrs, "; "))
		return
	}
	fmt.Fprintf(&w.body, "%s%s %s %s: %s { %s }\n", indent, from, arrow, to, d2Quote(label), strings.Join(attrs, "; "))
}

// link is the `link` attribute of a located origin's source site, empty when
// links are off or the origin has no site; D2 keeps it as the SVG anchor.
func (w *d2Writer) link(origin Origin) string {
	url, ok := w.links.URL(origin)
	if !ok {
		return ""
	}
	return "link: " + d2Quote(url)
}

// end is the path an edge ends at: the pin's where the edge names one that is
// drawn, else the node's.
func (w *d2Writer) end(node, port string) string {
	if path, ok := w.paths[port]; ok && port != "" {
		return path
	}
	if path, ok := w.paths[node]; ok {
		return path
	}
	return d2Key(node)
}

// d2Path is key's path under scope, the root scope being empty.
func d2Path(scope, key string) string {
	if scope == "" {
		return key
	}
	return scope + "." + key
}

// d2BareKey matches an identifier D2 reads as one key unquoted.
var d2BareKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// d2Reserved are D2's keywords, which a key must be quoted to spell.
var d2Reserved = map[string]bool{
	"shape": true, "label": true, "style": true, "direction": true, "near": true, "icon": true,
	"link": true, "tooltip": true, "width": true, "height": true, "constraint": true,
	"classes": true, "class": true, "vars": true, "layers": true, "scenarios": true, "steps": true,
	"top": true, "left": true, "grid-rows": true, "grid-columns": true, "grid-gap": true,
	"vertical-gap": true, "horizontal-gap": true, "source-arrowhead": true, "target-arrowhead": true,
}

// d2Key spells an ID as a D2 key: bare when D2 reads it as one, else quoted —
// a port's `node.port` ID would otherwise nest.
func d2Key(id string) string {
	if d2BareKey.MatchString(id) && !d2Reserved[id] {
		return id
	}
	return d2Quote(id)
}

// d2Escapes are the escapes a D2 double-quoted string needs: the quote and
// backslash, a line break, and the opening of a `${}` substitution.
var d2Escapes = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "${", `\${`)

// d2Quote writes text as a D2 double-quoted string.
func d2Quote(text string) string {
	return `"` + d2Escapes.Replace(text) + `"`
}
