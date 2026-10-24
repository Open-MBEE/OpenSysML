package view

import (
	"regexp"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// parameterSymbol reports whether node, nested in parent, is a parameter
// symbol: a case's or requirement's actor, subject or stakeholder drawn on it.
func parameterSymbol(parent, node *Node) bool {
	if parent == nil {
		return false
	}
	switch node.Kind {
	case "actor", "subject", "stakeholder":
	default:
		return false
	}
	return caseNodeKind(parent.Kind) || requirementNodeKind(parent.Kind)
}

// requirementNodeKind reports whether kind is a requirement or concern, whose
// subject and stakeholders the notation draws as parameters.
func requirementNodeKind(kind string) bool {
	kind = ast.NotationKeyword(kind)
	return kind == "requirement" || kind == "requirement def" || kind == "concern" || kind == "concern def"
}

// multiplicityText matches a multiplicity written after a name, `[4]` or
// `[0..*]`; a name holding brackets is quoted, so it ends in a quote instead.
var multiplicityText = regexp.MustCompile(`\[[^\]']*\]$`)

// showing is r drawn with the optional notation elements shown: its own Shown
// with more added. What is left out is taken off the rendering itself, so every
// form and the diagram data agree: parameter symbols and port symbols are
// removed and the edges and notes at them moved to the node that declared
// them, connector labels are blanked, types and multiplicities taken off.
// Keywords, compartments and qualifiers are the writers' to leave out, by
// the Shown the result carries. A rendering already showing the set is returned
// as is, and a run's: a run's drawing is no view notation and draws everything.
func (r *Rendering) showing(more Shown) *Rendering {
	if r.Run {
		return r
	}
	shown := r.Shown.Union(more)
	if shown == nil {
		shown = DefaultShown
	}
	if r.narrowed && len(more) == 0 {
		return r
	}
	out := r.Clone()
	out.Shown = shown
	out.narrowed = true
	n := narrowing{shown: shown, kind: r.Kind, redirect: map[string]string{}, ports: map[string]bool{}}
	out.Roots = n.nodes(out.Roots, nil)
	for i := range out.Edges {
		n.edge(&out.Edges[i])
	}
	for i := range out.Notes {
		n.note(&out.Notes[i])
	}
	return out
}

// narrowing is one pass over a rendering's nodes leaving out what is not shown.
type narrowing struct {
	shown Shown
	kind  Kind
	// redirect maps the ID of a removed node or port to the node that stands
	// for it: the nearest kept node above it.
	redirect map[string]string
	// ports marks the IDs redirect maps that were ports, which an edge names
	// as its FromPort or ToPort.
	ports map[string]bool
}

// nodes is nodes without the ones left out, each kept node narrowed in turn;
// parent is the kept node above them, nil at the roots.
func (n *narrowing) nodes(nodes []*Node, parent *Node) []*Node {
	var kept []*Node
	for _, node := range nodes {
		if n.leftOut(parent, node) {
			n.remove(node, parent)
			continue
		}
		n.narrow(node)
		node.Children = n.nodes(node.Children, node)
		kept = append(kept, node)
	}
	return kept
}

// leftOut reports whether node, nested in parent, is a parameter symbol the set
// leaves out: a case's parameter drawn on the case node of an interconnection,
// case or mixed diagram. A tree lists it as a member, which stays, as does a
// port drawn as a member; port symbols are a node's pins, narrow's concern.
func (n *narrowing) leftOut(parent, node *Node) bool {
	switch n.kind {
	case KindInterconnection, KindCase, KindMixed:
	default:
		return false
	}
	return parent != nil && !n.shown.Has(ElementParameterSymbols) && parameterSymbol(parent, node)
}

// remove maps node and everything nested in it to parent, the node now
// standing for them.
func (n *narrowing) remove(node, parent *Node) {
	n.redirect[node.ID] = parent.ID
	for _, port := range node.Ports {
		n.redirect[port.ID] = parent.ID
		n.ports[port.ID] = true
	}
	for _, child := range node.Children {
		n.remove(child, parent)
	}
}

// narrow leaves out of a kept node what is not shown: its parameter pins or
// port pins, and the types of it and of the pins kept.
func (n *narrowing) narrow(node *Node) {
	var ports []Port
	for _, port := range node.Ports {
		pinShown := n.shown.Has(ElementPortSymbols)
		if port.Parameter {
			pinShown = n.shown.Has(ElementParameterSymbols)
		}
		if !pinShown {
			n.redirect[port.ID] = node.ID
			n.ports[port.ID] = true
			continue
		}
		if !n.shown.Has(ElementTypings) {
			port.Type = ""
		}
		ports = append(ports, port)
	}
	node.Ports = ports
	if !n.shown.Has(ElementTypings) {
		node.Type = ""
	}
}

// edge moves an edge's ends off what was left out: an end at a removed port
// or node is at the node standing for it, and a connector's label is left
// out when edge labels are, its name with it, a usage's multiplicity when
// multiplicities are: an end name's always, the edge's own text's when it
// is a usage's name.
// A sequence diagram's message line is labelled by the message it is, which
// no element makes optional.
func (n *narrowing) edge(edge *Edge) {
	edge.From, edge.FromPort = n.end(edge.From, edge.FromPort)
	edge.To, edge.ToPort = n.end(edge.To, edge.ToPort)
	if !n.shown.Has(ElementEdgeLabels) && connectorEdge(edge.Kind) && n.kind != KindSequence {
		edge.Label, edge.Name = "", ""
	}
	if !n.shown.Has(ElementMultiplicities) {
		labels := []*string{&edge.FromLabel, &edge.ToLabel}
		if edge.Kind == EdgeComposition || edge.Kind == EdgeReference {
			labels = append(labels, &edge.Label)
		}
		for _, label := range labels {
			*label = multiplicityText.ReplaceAllString(*label, "")
		}
	}
}

// end is where an edge end at node and port is drawn now: the port when it is
// kept, else the node standing for the port or for the node.
func (n *narrowing) end(node, port string) (string, string) {
	if port != "" {
		if to, ok := n.redirect[port]; ok && n.ports[port] {
			return to, ""
		}
	}
	if to, ok := n.redirect[node]; ok {
		return to, ""
	}
	return node, port
}

// note moves a note anchored at a removed node to the node standing for it.
func (n *narrowing) note(note *Note) {
	for _, anchor := range []*string{&note.Anchor, &note.EdgeFrom, &note.EdgeTo} {
		if to, ok := n.redirect[*anchor]; ok && *anchor != "" {
			*anchor = to
		}
	}
}

// connectorEdge reports whether an edge stands for a connector whose name is
// an optional label: a connection, interface, flow, binding or allocation.
func connectorEdge(kind EdgeKind) bool {
	switch kind {
	case EdgeConnection, EdgeFlow, EdgeBinding, EdgeAllocate:
		return true
	}
	return false
}
