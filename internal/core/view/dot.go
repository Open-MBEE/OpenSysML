package view

import (
	"fmt"
	"strings"
)

// DOT is the Graphviz form of a graph-shaped rendering: a `digraph` for the
// `dot` engine, written as text like the Mermaid form, with no Graphviz
// installed. A node with children is a `subgraph "cluster_<id>"` (a tree draws
// containment as edges, as its Mermaid form does); an edge ending at a cluster
// is drawn to a node inside it and clipped with `lhead`/`ltail`. Edge kinds
// parallel the Mermaid arrows:
//
//	EdgeKind        Mermaid  DOT
//	EdgeConnection  ---      arrowhead=none
//	EdgeTransition  -->      solid, default arrowhead
//	EdgeSuccession  -->      solid, default arrowhead
//	EdgeFlow        -.->     style=dashed
//	(containment)   ---      arrowhead=none
//
// A kind with no DOT counterpart — sequence, table — is a *WrongFormError.
func (r *Rendering) DOT() (string, error) {
	return r.DOTDirected("")
}

// DOTDirected is the DOT form laid out in the stated direction, as `rankdir`;
// the empty direction leaves the engine's default and writes no `rankdir`.
func (r *Rendering) DOTDirected(direction Direction) (string, error) {
	if !r.Kind.SupportsForm(FormDot) {
		return "", &WrongFormError{Form: FormDot, Kind: r.Kind, View: r.View}
	}
	w := &dotWriter{tree: r.Kind == KindTree, clusters: map[string]dotCluster{}}
	if !w.tree {
		for _, root := range r.Roots {
			w.collectClusters(root)
		}
	}
	for _, edge := range r.Edges {
		if _, ok := w.clusters[edge.From]; ok {
			w.compound = true
		}
		if _, ok := w.clusters[edge.To]; ok {
			w.compound = true
		}
	}
	b := &w.b
	if r.View != "" {
		fmt.Fprintf(b, "// view: %s\n", r.View)
	}
	fmt.Fprintf(b, "// kind: %s\n", r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(b, "// stated: %s\n", r.Stated)
	}
	for _, notice := range r.Notices {
		fmt.Fprintf(b, "// not represented: %s\n", notice)
	}
	b.WriteString("// layout: dot\n")
	if r.View == "" {
		b.WriteString("digraph {\n")
	} else {
		fmt.Fprintf(b, "digraph %s {\n", dotQuote(r.View))
	}
	if attrs := w.graphAttributes(direction); len(attrs) > 0 {
		fmt.Fprintf(b, "  graph [%s];\n", strings.Join(attrs, ", "))
	}
	b.WriteString("  node [shape=box];\n")
	if r.Empty() {
		fmt.Fprintf(b, "  \"empty\" [shape=plaintext, label=%s];\n", dotQuote(r.EmptyReason()))
		b.WriteString("}\n")
		return b.String(), nil
	}
	for _, root := range r.Roots {
		w.writeNode(root, 1)
	}
	for _, edge := range r.Edges {
		w.writeEdge(edge.From, edge.To, dotEdgeAttributes(edge))
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// dotWriter holds what one rendering's DOT form needs across nodes and edges.
type dotWriter struct {
	b        strings.Builder
	tree     bool                  // containment as edges, not clusters
	clusters map[string]dotCluster // node ID -> the cluster drawn for it
	compound bool                  // an edge is clipped at a cluster
}

// dotCluster is a node drawn as a cluster: its name and the first leaf inside
// it, which stands in for the cluster as an edge endpoint.
type dotCluster struct {
	name   string
	anchor string
}

// collectClusters records every node under node that is drawn as a cluster.
func (w *dotWriter) collectClusters(node *Node) {
	if len(node.Children) == 0 {
		return
	}
	w.clusters[node.ID] = dotCluster{name: "cluster_" + node.ID, anchor: firstLeaf(node).ID}
	for _, child := range node.Children {
		w.collectClusters(child)
	}
}

// firstLeaf is the first childless node under node, in rendering order.
func firstLeaf(node *Node) *Node {
	for len(node.Children) > 0 {
		node = node.Children[0]
	}
	return node
}

// graphAttributes is the graph attribute list: the layout direction when one is
// stated, and `compound` when an edge is clipped at a cluster.
func (w *dotWriter) graphAttributes(direction Direction) []string {
	var attrs []string
	if direction != "" {
		attrs = append(attrs, "rankdir="+string(direction))
	}
	if w.compound {
		attrs = append(attrs, "compound=true")
	}
	return attrs
}

// writeNode writes one node: a cluster holding its children, a plain node
// otherwise. A tree writes the node and an edge to each child instead.
func (w *dotWriter) writeNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 || w.tree {
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(node.ID), strings.Join(dotNodeAttributes(node), ", "))
		for _, child := range node.Children {
			w.writeNode(child, depth)
			w.writeEdge(node.ID, child.ID, dotContainmentAttributes())
		}
		return
	}
	fmt.Fprintf(&w.b, "%ssubgraph %s {\n", indent, dotQuote(w.clusters[node.ID].name))
	for _, attr := range dotClusterAttributes(node) {
		fmt.Fprintf(&w.b, "%s  %s;\n", indent, attr)
	}
	for _, child := range node.Children {
		w.writeNode(child, depth+1)
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeEdge writes one edge. An end that is a cluster is drawn to the cluster's
// anchor node and clipped at the cluster with `ltail`/`lhead`.
func (w *dotWriter) writeEdge(from, to string, attrs []string) {
	if cluster, ok := w.clusters[from]; ok {
		from = cluster.anchor
		attrs = append(attrs, "ltail="+dotQuote(cluster.name))
	}
	if cluster, ok := w.clusters[to]; ok {
		to = cluster.anchor
		attrs = append(attrs, "lhead="+dotQuote(cluster.name))
	}
	if len(attrs) == 0 {
		fmt.Fprintf(&w.b, "  %s -> %s;\n", dotQuote(from), dotQuote(to))
		return
	}
	fmt.Fprintf(&w.b, "  %s -> %s [%s];\n", dotQuote(from), dotQuote(to), strings.Join(attrs, ", "))
}

// dotNodeAttributes is a plain node's attribute list: its label and the shape
// its Kind chooses. A position joins the list once a node carries geometry.
func dotNodeAttributes(node *Node) []string {
	switch node.Kind {
	case startKind:
		return []string{"shape=point", `label=""`}
	case "initial":
		return []string{"shape=circle", "label=" + dotLabel(node)}
	case "final":
		return []string{"shape=doublecircle", "label=" + dotLabel(node)}
	case "state":
		return []string{"shape=box", "style=rounded", "label=" + dotLabel(node)}
	}
	return []string{"label=" + dotLabel(node)}
}

// dotClusterAttributes is a cluster's attribute statements: its label, and a
// dashed border for an orthogonal region.
func dotClusterAttributes(node *Node) []string {
	attrs := []string{"label=" + dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	return attrs
}

// dotEdgeAttributes is an edge's attribute list: its label and its kind's
// style. A route joins the list once an edge carries waypoints.
func dotEdgeAttributes(edge Edge) []string {
	var attrs []string
	if edge.Label != "" {
		attrs = append(attrs, "label="+dotQuote(edge.Label))
	}
	switch edge.Kind {
	case EdgeConnection:
		attrs = append(attrs, "arrowhead=none")
	case EdgeFlow:
		attrs = append(attrs, "style=dashed")
	}
	return attrs
}

// dotContainmentAttributes is a tree's containment edge, Mermaid's `---`.
func dotContainmentAttributes() []string {
	return []string{"arrowhead=none"}
}

// dotLabel is a node's quoted label: kind and name, then its detail on a
// second line.
func dotLabel(node *Node) string {
	head := node.Kind
	if node.Name != "" {
		head += " " + node.Name
	}
	if node.Detail == "" {
		return dotQuote(head)
	}
	return dotQuote(head + "\n" + node.Detail)
}

// dotQuote writes text as a double-quoted DOT string; every ID and label goes
// through it, so none is written bare. A newline becomes a `\n` line break.
func dotQuote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}
