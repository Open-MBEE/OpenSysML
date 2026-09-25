package view

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// renderInterconnection renders the exposed features as nodes, the features
// nested in them as nested nodes, and the connections between them as edges.
// The connections are the connector usages the model holds, resolved through the
// same connector-end information an object of a connector is materialized from —
// never re-derived from the source text. Nodes and edges are placed by the
// Layout and Route annotations positioning their elements in view, nil for a
// rendering outside any view. An exposed feature another exposed feature draws
// nested in it is not a second root.
func (r *Renderer) renderInterconnection(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	w := &featureWalk{r: r, view: view, ids: &nodeIDs{}, nodes: map[*symbols.Symbol]*Node{}, out: out}
	descendants := r.exposedDescendants(exposed, r.interconnectionMembers)
	for _, elem := range exposed {
		switch {
		case r.drawsConnector(elem):
			w.connectors = append(w.connectors, elem)
		case featureLike(elem):
			if descendants[symbols.KeyOf(elem)] {
				continue
			}
			out.Roots = append(out.Roots, w.featureNode(elem, map[*symbols.Symbol]bool{}, 0, true))
		default:
			out.Notices = append(out.Notices, fmt.Sprintf(
				"%s %s has no place in an interconnection rendering; it is not shown",
				declKind(elem), r.notationName(elem)))
		}
	}
	seen := map[*symbols.Symbol]bool{}
	for _, connector := range w.connectors {
		if seen[connector] {
			continue
		}
		seen[connector] = true
		r.connectionEdges(view, connector, w.nodes, out)
	}
}

// featureWalk is one interconnection rendering's walk over the exposed features:
// the nodes rendered so far and the connectors collected along the way.
type featureWalk struct {
	r          *Renderer
	view       *symbols.Symbol
	ids        *nodeIDs
	nodes      map[*symbols.Symbol]*Node
	connectors []*symbols.Symbol
	out        *Rendering
}

// featureNode renders one exposed feature and the features nested in it,
// collecting the connectors declared along the way, which join the nodes it
// renders.
func (w *featureWalk) featureNode(sym *symbols.Symbol, seen map[*symbols.Symbol]bool, depth int, qualified bool) *Node {
	r := w.r
	name := r.notationName(sym)
	if !qualified {
		name = localName(sym)
	}
	node := &Node{ID: w.ids.take(), Kind: declKind(sym), Name: name, NameSynthesized: r.model.NameSynthesized(sym),
		Type: declType(sym), Typings: r.declTypings(sym), Origin: symbolOrigin(sym), Geometry: r.geometryOf(w.view, sym, w.out),
		Style: r.styleOf(w.view, sym, w.out)}
	if existing, ok := w.nodes[sym]; ok {
		node.Detail = detailWith(node.Detail, "already shown as "+existing.ID)
		return node
	}
	w.nodes[sym] = node
	r.notesOf(w.view, sym, node.ID, w.out)
	if seen[sym] || depth >= r.treeDepth() {
		return node
	}
	seen[sym] = true
	for _, member := range r.containedMembers(sym) {
		switch {
		case r.drawsConnector(member):
			w.connectors = append(w.connectors, member)
		case featureLike(member):
			node.Children = append(node.Children, w.featureNode(member, seen, depth+1, false))
		}
	}
	return node
}

// interconnectionMembers is the members featureNode draws as nested nodes:
// the feature-like of what an element declares, connectors being edges.
func (r *Renderer) interconnectionMembers(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range r.containedMembers(sym) {
		if featureLike(member) {
			out = append(out, member)
		}
	}
	return out
}

// connectionEdges adds the edges one connector or flow contributes. A binary
// connector joins its two ends; a multi-end connector makes every end reachable
// from every other, so each pair is an edge. An end attaching to something the
// view does not expose is reported rather than dropped.
func (r *Renderer) connectionEdges(view, connector *symbols.Symbol, nodes map[*symbols.Symbol]*Node, out *Rendering) {
	label := r.connectorLabel(connector)
	ends, kind := r.connectorEnds(connector)
	if len(ends) < 2 {
		out.Notices = append(out.Notices, fmt.Sprintf("%s %s joins fewer than two features; no edge is drawn",
			declKind(connector), r.notationName(connector)))
		return
	}
	resolved := make([]*Node, 0, len(ends))
	for _, end := range ends {
		node := r.endNode(connector, end, nodes)
		if node == nil {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s attaches to %s, which the view does not expose; no edge is drawn",
				declKind(connector), r.notationName(connector), notationName(end.path)))
			return
		}
		resolved = append(resolved, node)
	}
	route, style := r.routeOf(view, connector, out), r.edgeDress(view, connector, resolved[0].ID, resolved[1].ID, out)
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			out.Edges = append(out.Edges, Edge{
				From: resolved[i].ID, To: resolved[j].ID, Label: label, Kind: kind,
				Origin: symbolOrigin(connector), Route: slices.Clone(route), Style: style,
			})
		}
	}
}

// connectorEnd is one end of a connection as the rendering reads it: the node
// naming what it attaches to, and that name as the model wrote it.
type connectorEnd struct {
	attachment ast.Node
	path       string
}

// drawsConnector reports whether an interconnection rendering draws sym as an
// edge: a connector usage, a binding, or a flow between features.
func (r *Renderer) drawsConnector(sym *symbols.Symbol) bool {
	return r.model.IsConnectorUsage(sym) || isBindingUsage(sym) || isFlowUsage(sym)
}

// connectorEnds returns the ends of a connector, binding or flow usage and the edge kind it
// makes, read from the model's connector information so redefined inherited ends resolve.
func (r *Renderer) connectorEnds(connector *symbols.Symbol) ([]connectorEnd, EdgeKind) {
	if isBindingUsage(connector) {
		var out []connectorEnd
		for _, att := range r.model.ConnectorObjectEnds(connector) {
			if att.Attachment == nil {
				continue
			}
			out = append(out, connectorEnd{attachment: att.Attachment, path: lower.FeaturePath(att.Attachment)})
		}
		return out, EdgeBinding
	}
	if isFlowUsage(connector) {
		usage, _ := connector.Decl.(*ast.Usage)
		if usage == nil || usage.FlowEnds == nil {
			return nil, EdgeFlow
		}
		var out []connectorEnd
		for _, end := range []ast.Node{usage.FlowEnds.From, usage.FlowEnds.To} {
			if end == nil {
				continue
			}
			out = append(out, connectorEnd{attachment: end, path: lower.FeaturePath(end)})
		}
		return out, EdgeFlow
	}
	var out []connectorEnd
	for _, att := range r.model.ConnectorEndAttachments(connector) {
		if att.Attachment == nil {
			continue
		}
		out = append(out, connectorEnd{attachment: att.Attachment, path: lower.FeaturePath(att.Attachment)})
	}
	return out, EdgeConnection
}

// endNode is the node an end attaches to: the feature it names when the
// rendering shows it, else the nearest feature owning it that the rendering
// shows — a port of a part the view exposes without exposing the port itself.
func (r *Renderer) endNode(connector *symbols.Symbol, end connectorEnd, nodes map[*symbols.Symbol]*Node) *Node {
	// A chain is tried whole first, then a segment shorter: `pump.out` attaches
	// to a port of a part, and the part is what a view exposing the enclosing
	// definition shows.
	for attachment := end.attachment; attachment != nil; attachment = chainOperand(attachment) {
		target, ok := r.resolver.ResolveTarget(connector.OwnerScope, attachment)
		if !ok {
			continue
		}
		for sym := target; sym != nil; sym = sym.Owner() {
			if node, ok := nodes[sym]; ok {
				return node
			}
		}
	}
	return nil
}

// chainOperand is the feature the last segment of a feature chain is a member
// of, nil for a target that is no chain.
func chainOperand(node ast.Node) ast.Node {
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		return chain.Operand
	}
	return nil
}

// connectorLabel names a connection on an edge: the payload it carries, else its
// own name, else the type it is declared with, else the keyword that declared it.
// A name a migration made up counts as none, as for the unnamed source connector.
func (r *Renderer) connectorLabel(connector *symbols.Symbol) string {
	if payload := flowPayload(connector); payload != "" {
		return "of " + notationName(payload)
	}
	if connector.Name != "" && !connector.EffectiveName() && !r.model.NameSynthesized(connector) {
		return localName(connector)
	}
	if declared := declType(connector); declared != "" {
		return declared
	}
	return declKind(connector)
}

// flowPayload is what a flow carries, as written, empty for a flow declaring no
// payload and for a symbol that is no flow.
func flowPayload(sym *symbols.Symbol) string {
	if !isFlowUsage(sym) {
		return ""
	}
	usage, _ := sym.Decl.(*ast.Usage)
	return qualifiedText(usage.FlowEnds.Payload)
}

// isBindingUsage reports whether a symbol is a binding equating two features,
// which is an edge of an interconnection rendering.
func isBindingUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageBinding && len(usage.ConnectorEnds) == 2
}

// isFlowUsage reports whether a symbol is a flow usage stating the features it
// flows between, which is an edge of an interconnection rendering.
func isFlowUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageFlow && usage.FlowEnds != nil
}

// featureLike reports whether an element is a feature an interconnection
// rendering shows as a node: the structural elements a system is built from.
// Behaviors, views and namespaces are not, and are reported rather than drawn.
func featureLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolPortDef, symbols.SymbolPortUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage,
		symbols.SymbolAttributeDef, symbols.SymbolAttributeUsage,
		symbols.SymbolEnumerationDef, symbols.SymbolEnumerationUsage,
		symbols.SymbolInterfaceDef, symbols.SymbolConnectionDef, symbols.SymbolAllocationDef,
		symbols.SymbolKerMLType:
		return true
	}
	return false
}
