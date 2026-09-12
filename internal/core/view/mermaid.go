package view

import (
	"fmt"
	"strings"
)

// Mermaid is the default machine-readable form of a rendering: a Mermaid
// diagram renders where the models are read — in Markdown documentation, in
// the repository's own docs, and in the editors that host the language server —
// without a Graphviz installation, and it has a state-diagram grammar the state
// rendering maps onto directly. Graphviz DOT, written by DOT, is the alternative
// for Graphviz toolchains and for renderings that will carry exact positions.
//
// A graph-shaped rendering is a `flowchart`; a state rendering is a
// `stateDiagram-v2` and a sequence rendering a `sequenceDiagram`. What the
// rendering could not represent is written as comments, so no notice is lost in
// the machine-readable form either.
func (r *Rendering) Mermaid() string {
	return r.MermaidDirected("")
}

// MermaidDirected is the Mermaid form drawn in the stated direction: a
// flowchart flows that way, and a state diagram states it as a `direction`
// statement. The empty direction keeps each kind's default, and a kind no
// direction applies to ignores it.
func (r *Rendering) MermaidDirected(direction Direction) string {
	var b strings.Builder
	if r.View == "" {
		fmt.Fprintf(&b, "%%%% %s rendering", r.Kind)
	} else {
		fmt.Fprintf(&b, "%%%% %s — %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	for _, notice := range r.Notices {
		fmt.Fprintf(&b, "%%%% not represented: %s\n", notice)
	}
	switch r.Kind {
	case KindState:
		r.writeStateDiagram(&b, direction)
		return b.String()
	case KindSequence:
		r.writeSequenceDiagram(&b)
		return b.String()
	}
	r.writeFlowchart(&b, direction)
	return b.String()
}

// writeFlowchart writes the tree, interconnection and action renderings as a
// Mermaid flowchart: a node with children is a subgraph, containment in a tree
// is an edge, and every other edge is the one the rendering holds.
func (r *Rendering) writeFlowchart(b *strings.Builder, direction Direction) {
	flow := "TD"
	if r.Kind == KindInterconnection {
		flow = "LR"
	}
	if direction != "" {
		flow = string(direction)
	}
	fmt.Fprintf(b, "flowchart %s\n", flow)
	if r.Empty() {
		fmt.Fprintf(b, "  empty[\"%s\"]\n", mermaidText(r.EmptyReason()))
		return
	}
	for _, root := range r.Roots {
		writeFlowchartNode(b, root, 1, r.Kind == KindTree)
	}
	for _, edge := range r.Edges {
		if edge.Label == "" {
			fmt.Fprintf(b, "  %s %s %s\n", edge.From, mermaidArrow(edge.Kind), edge.To)
			continue
		}
		fmt.Fprintf(b, "  %s %s|\"%s\"| %s\n", edge.From, mermaidArrow(edge.Kind), mermaidText(edge.Label), edge.To)
	}
}

// writeFlowchartNode writes one node: a subgraph when it holds others, a plain
// node otherwise. containment adds an edge from a node to each of its children,
// which is how a tree rendering shows what contains what.
func writeFlowchartNode(b *strings.Builder, node *Node, depth int, containment bool) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 {
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
		return
	}
	if containment {
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
		for _, child := range node.Children {
			writeFlowchartNode(b, child, depth, containment)
			fmt.Fprintf(b, "%s%s --- %s\n", indent, node.ID, child.ID)
		}
		return
	}
	fmt.Fprintf(b, "%ssubgraph %s [\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
	for _, child := range node.Children {
		writeFlowchartNode(b, child, depth+1, containment)
	}
	fmt.Fprintf(b, "%send\n", indent)
}

// writeStateDiagram writes a state rendering as a Mermaid state diagram: bodies
// are composite states, entry transitions leave the `[*]` marker of their body.
func (r *Rendering) writeStateDiagram(b *strings.Builder, direction Direction) {
	b.WriteString("stateDiagram-v2\n")
	if direction != "" {
		fmt.Fprintf(b, "  direction %s\n", direction)
	}
	if r.Empty() {
		// A state diagram takes a note only attached to a state, so the reason
		// is a state of its own.
		fmt.Fprintf(b, "  state \"%s\" as empty\n", mermaidText(r.EmptyReason()))
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
		writeStateNode(b, root, 1, starts)
	}
	for _, edge := range r.Edges {
		if _, ok := starts[edge.From]; ok {
			continue
		}
		writeStateEdge(b, edge.From, edge.To, edge.Label, 1)
	}
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
	fmt.Fprintf(b, "%s%s --> %s : %s\n", indent, from, to, mermaidText(label))
}

// writeSequenceDiagram writes a sequence rendering as a Mermaid sequence
// diagram: one participant per lifeline, declared before the messages, then the
// messages in the order the rendering settled on.
func (r *Rendering) writeSequenceDiagram(b *strings.Builder) {
	b.WriteString("sequenceDiagram\n")
	if r.Empty() {
		// A sequence diagram carries no free text, so the reason is a
		// participant of its own.
		fmt.Fprintf(b, "  participant empty as %s\n", mermaidText(r.EmptyReason()))
		return
	}
	for _, node := range r.Roots {
		fmt.Fprintf(b, "  participant %s as %s\n", node.ID, mermaidText(mermaidLabel(node)))
	}
	for _, edge := range r.Edges {
		// The colon is part of the message syntax; only the text after it is left
		// off when the message carries none.
		if edge.Label == "" {
			fmt.Fprintf(b, "  %s->>%s:\n", edge.From, edge.To)
			continue
		}
		fmt.Fprintf(b, "  %s->>%s: %s\n", edge.From, edge.To, mermaidText(edge.Label))
	}
}

// writeStateNode writes one state and its substates. A body's start is the `[*]`
// marker inside that state, so its edges are written there after the substates.
func writeStateNode(b *strings.Builder, node *Node, depth int, starts map[string][]Edge) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 {
		fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, mermaidText(mermaidLabel(node)), node.ID)
		return
	}
	fmt.Fprintf(b, "%sstate \"%s\" as %s {\n", indent, mermaidText(mermaidLabel(node)), node.ID)
	for _, child := range node.Children {
		if child.Kind != startKind {
			writeStateNode(b, child, depth+1, starts)
		}
	}
	for _, child := range node.Children {
		for _, edge := range starts[child.ID] {
			writeStateEdge(b, "[*]", edge.To, edge.Label, depth+1)
		}
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

// mermaidLabel is the text a node carries in a diagram: its kind, its name, and
// what else the rendering said about it.
func mermaidLabel(node *Node) string {
	label := node.Kind
	if node.Name != "" {
		label += " " + node.Name
	}
	if node.Detail != "" {
		label += " (" + node.Detail + ")"
	}
	return label
}

// mermaidArrow is how an edge of each kind is drawn in a flowchart.
func mermaidArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "---"
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
