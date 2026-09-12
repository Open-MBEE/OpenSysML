package view

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// renderInterconnection renders the exposed features as nodes, the features
// nested in them as nested nodes, and the connections between them as edges.
// The connections are the connector usages the model holds, resolved through the
// same connector-end information an object of a connector is materialized from —
// never re-derived from the source text. Nodes and edges are placed by the
// Layout and Route annotations positioning their elements in view, nil for a
// rendering outside any view.
func (r *Renderer) renderInterconnection(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	nodes := map[*symbols.Symbol]*Node{}
	var connectors []*symbols.Symbol
	for _, elem := range exposed {
		switch {
		case r.model.IsConnectorUsage(elem), isFlowUsage(elem):
			connectors = append(connectors, elem)
		case featureLike(elem):
			out.Roots = append(out.Roots, r.featureNode(view, elem, ids, nodes, &connectors, map[*symbols.Symbol]bool{}, 0, true, out))
		default:
			out.Notices = append(out.Notices, fmt.Sprintf(
				"%s %s has no place in an interconnection rendering; it is not shown",
				declKind(elem), r.notationName(elem)))
		}
	}
	seen := map[*symbols.Symbol]bool{}
	for _, connector := range connectors {
		if seen[connector] {
			continue
		}
		seen[connector] = true
		r.connectionEdges(view, connector, nodes, out)
	}
}

// featureNode renders one exposed feature and the features nested in it,
// collecting the connectors declared along the way, which join the nodes it
// renders.
func (r *Renderer) featureNode(view, sym *symbols.Symbol, ids *nodeIDs, nodes map[*symbols.Symbol]*Node,
	connectors *[]*symbols.Symbol, seen map[*symbols.Symbol]bool, depth int, qualified bool, out *Rendering) *Node {
	name := r.notationName(sym)
	if !qualified {
		name = notationName(simpleName(r.fqn(sym)))
	}
	node := &Node{ID: ids.take(), Kind: declKind(sym), Name: name, Detail: declType(sym), Origin: symbolOrigin(sym),
		Geometry: r.geometryOf(view, sym, out)}
	if existing, ok := nodes[sym]; ok {
		node.Detail = detailWith(node.Detail, "already shown as "+existing.ID)
		return node
	}
	nodes[sym] = node
	if seen[sym] || depth >= maxTreeDepth {
		return node
	}
	seen[sym] = true
	for _, member := range containedMembers(sym) {
		switch {
		case r.model.IsConnectorUsage(member), isFlowUsage(member):
			*connectors = append(*connectors, member)
		case featureLike(member):
			node.Children = append(node.Children, r.featureNode(view, member, ids, nodes, connectors, seen, depth+1, false, out))
		}
	}
	return node
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
	route := r.routeOf(view, connector, out)
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			out.Edges = append(out.Edges, Edge{
				From: resolved[i].ID, To: resolved[j].ID, Label: label, Kind: kind, Origin: symbolOrigin(connector),
				Route: slices.Clone(route),
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

// connectorEnds returns the ends of a connector or flow usage, and what kind of
// edge it makes. The ends come from the model's own connector information, so an
// end that redefines an inherited one is read the way an object of the connector
// reads it.
func (r *Renderer) connectorEnds(connector *symbols.Symbol) ([]connectorEnd, EdgeKind) {
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
		for sym := target; sym != nil; sym = ownerOf(sym) {
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

// ownerOf is the element a symbol was declared in, nil for one declared at the
// top of a document.
func ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// connectorLabel names a connection on an edge: its own name, else the type it
// is declared with, else the keyword that declared it.
func (r *Renderer) connectorLabel(connector *symbols.Symbol) string {
	if name := simpleName(r.fqn(connector)); name != "" && !connector.EffectiveName() {
		return notationName(name)
	}
	if declared := declType(connector); declared != "" {
		return notationName(declared)
	}
	if payload := flowPayload(connector); payload != "" {
		return "of " + notationName(payload)
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
