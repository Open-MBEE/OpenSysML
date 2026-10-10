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
	r.renderInterconnectionWithIDs(view, exposed, out, &nodeIDs{}, false)
}

// renderInterconnectionWithIDs walks structural features with shared node IDs.
func (r *Renderer) renderInterconnectionWithIDs(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering, ids *nodeIDs, mixed bool) map[*symbols.Symbol]*Node {
	w := &featureWalk{r: r, view: view, ids: ids, nodes: map[*symbols.Symbol]*Node{},
		pins: map[*Node]map[*symbols.Symbol]string{}, parent: map[*Node]*Node{}, out: out, mixed: mixed}
	var roots []*symbols.Symbol
	for _, elem := range exposed {
		if !r.drawsConnector(elem) && w.drawsFeature(elem) {
			roots = append(roots, elem)
		}
	}
	descendants := r.exposedDescendants(roots, w.nestedFeatures)
	for _, elem := range exposed {
		switch {
		case r.drawsConnector(elem):
			w.connectors = append(w.connectors, elem)
		case w.drawsFeature(elem):
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
		w.connectionEdges(connector)
	}
	w.valueBindingEdges()
	return w.nodes
}

// featureWalk is one interconnection rendering's walk over the exposed features:
// the nodes rendered so far, by the feature each draws, and the connectors
// collected along the way. pins is the ports drawn on the border of a node
// rather than as nodes of their own — the ports a feature has from its type
// without declaring them, and a case's parameters — by node and port; drawn is
// every feature drawn, as a node or a pin, in drawing order; parent is the node
// each nested node is drawn in.
type featureWalk struct {
	r          *Renderer
	view       *symbols.Symbol
	ids        *nodeIDs
	nodes      map[*symbols.Symbol]*Node
	drawn      []*symbols.Symbol
	pins       map[*Node]map[*symbols.Symbol]string
	parent     map[*Node]*Node
	connectors []*symbols.Symbol
	out        *Rendering
	mixed      bool
}

// drawsFeature reports whether the interconnection traversal emits sym as a feature node.
func (w *featureWalk) drawsFeature(sym *symbols.Symbol) bool {
	if !featureLike(sym) {
		return false
	}
	if !w.mixed {
		return true
	}
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage, symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolPortDef, symbols.SymbolPortUsage, symbols.SymbolConnectionDef, symbols.SymbolConnectionUsage,
		symbols.SymbolInterfaceDef, symbols.SymbolInterfaceUsage, symbols.SymbolAllocationDef, symbols.SymbolAllocationUsage:
		return true
	}
	return false
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
	w.drawn = append(w.drawn, sym)
	w.pinPorts(node, sym)
	r.notesOf(w.view, sym, node.ID, w.out)
	if seen[sym] || depth >= r.treeDepth() {
		return node
	}
	seen[sym] = true
	for _, member := range r.containedMembers(sym) {
		switch {
		case r.drawsConnector(member):
			w.connectors = append(w.connectors, member)
		case casePin(sym, member):
			w.pin(node, member, pinDirection(member))
		case isReferencedIncludedCase(member):
			// an `include u;` is drawn by the connection to the case it names
		case w.drawsFeature(member):
			child := w.featureNode(member, seen, depth+1, false)
			w.parent[child] = node
			node.Children = append(node.Children, child)
		}
	}
	return node
}

// pinPorts draws on node's border the ports sym has from its type, each named
// and typed as the type declares it: `heating : HeatingSystem` shows the
// `durationIn : ~DurationPort` HeatingSystem declares, and a connector ending
// at `heating.durationIn` ends at that pin.
func (w *featureWalk) pinPorts(node *Node, sym *symbols.Symbol) {
	for _, port := range w.r.typedPorts(sym) {
		w.pin(node, port, PortUndirected)
	}
}

// pin draws sym as a port on node's border, with the given direction.
func (w *featureWalk) pin(node *Node, sym *symbols.Symbol, direction PortDirection) {
	id := portID(node.ID, len(node.Ports))
	node.Ports = append(node.Ports, Port{ID: id, Name: localName(sym), Type: declType(sym),
		Direction: direction, Origin: symbolOrigin(sym)})
	if w.pins[node] == nil {
		w.pins[node] = map[*symbols.Symbol]string{}
	}
	w.pins[node][sym] = id
	w.drawn = append(w.drawn, sym)
}

// siteOf is where a drawn feature is: its own node, or the node it is a pin of
// and the pin; the zero edgeEnd for one not drawn.
func (w *featureWalk) siteOf(sym *symbols.Symbol) edgeEnd {
	if node := w.nodes[sym]; node != nil {
		return edgeEnd{node: node}
	}
	for node, pins := range w.pins {
		if id, ok := pins[sym]; ok {
			return edgeEnd{node: node, port: id}
		}
	}
	return edgeEnd{}
}

// casePin reports whether member, a member of an analysis case drawn as a
// node, is drawn as a pin on its border rather than a node inside it: the
// case's subject, parameters and attributes, as the graphical notation draws
// a case's parameters. Its parts, actions and nested cases are not.
func casePin(owner, member *symbols.Symbol) bool {
	switch owner.Kind {
	case symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage:
	default:
		return false
	}
	switch member.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage:
		return true
	}
	usage, ok := member.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageSubject
}

// pinDirection is the direction a parameter is declared with: `return` and
// `out` give values out, `in` takes them in; a subject or attribute has none.
func pinDirection(sym *symbols.Symbol) PortDirection {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return PortUndirected
	}
	switch {
	case usage.IsResult, usage.Direction == ast.DirOut:
		return PortOut
	case usage.Direction == ast.DirIn:
		return PortIn
	case usage.Direction == ast.DirInOut:
		return PortInOut
	}
	return PortUndirected
}

// typedPorts is the ports a feature has without declaring them: those of the
// definition typing it and of what that specializes, less any a port of its own
// redefines. They are its boundary, where an interconnection's connectors
// attach. The ports the library gives every part and port — `ownedPorts`,
// `subports`, `interfacingPorts` — are not among them: they are the notation's
// vocabulary, not the model's.
func (r *Renderer) typedPorts(sym *symbols.Symbol) []*symbols.Symbol {
	owned := map[*symbols.Symbol]bool{}
	for _, member := range r.containedMembers(sym) {
		owned[member] = true
	}
	var out []*symbols.Symbol
	for _, member := range r.model.MembersOf(sym) {
		if member.Kind != symbols.SymbolPortUsage || owned[member] || member.Owner() == sym ||
			!r.contentKind(member) || r.libraryDeclared(member) {
			continue
		}
		out = append(out, member)
	}
	return out
}

// libraryDeclared reports whether the bundled standard library declares sym.
func (r *Renderer) libraryDeclared(sym *symbols.Symbol) bool {
	idx := r.resolver.Index()
	return idx != nil && idx.Library(sym)
}

// nestedFeatures is the members featureNode draws as nested nodes of sym:
// the features this walk draws, connectors being edges.
func (w *featureWalk) nestedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range w.r.containedMembers(sym) {
		if w.drawsFeature(member) && !isReferencedIncludedCase(member) {
			out = append(out, member)
		}
	}
	return out
}

// connectionEdges adds the edges one connector or flow contributes. A binary
// connector joins its two ends; a multi-end connector makes every end reachable
// from every other, so each pair is an edge. An end attaching to something the
// view does not expose is reported rather than dropped.
func (w *featureWalk) connectionEdges(connector *symbols.Symbol) {
	r, out := w.r, w.out
	label := r.connectorLabel(connector)
	ends, kind := r.connectorEnds(connector)
	if len(ends) < 2 {
		out.Notices = append(out.Notices, fmt.Sprintf("%s %s joins fewer than two features; no edge is drawn",
			declKind(connector), r.notationName(connector)))
		return
	}
	resolved := make([]edgeEnd, 0, len(ends))
	for _, end := range ends {
		at := w.endNode(connector, end.attachment)
		if at.node == nil {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s attaches to %s, which the view does not expose; no edge is drawn",
				declKind(connector), r.notationName(connector), notationName(end.path)))
			return
		}
		resolved = append(resolved, at)
	}
	route, style := r.routeOf(w.view, connector, out), r.edgeDress(w.view, connector, resolved[0].node.ID, resolved[1].node.ID, out)
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			out.Edges = append(out.Edges, Edge{
				From: resolved[i].node.ID, To: resolved[j].node.ID, FromPort: resolved[i].port, ToPort: resolved[j].port,
				Label: label, Kind: kind, Origin: symbolOrigin(connector), Route: slices.Clone(route), Style: style,
			})
		}
	}
}

// valueBindingEdges draws each drawn feature whose value names another feature
// (`attribute :>> observed = analysed.t`, a binding per KerML 7.4.9) as a binding
// edge. The value attaches where a connector end naming it would — `second.y` is
// the y under second's node, not its type's — and only when that is the valued
// feature's own node (the subject pinned on it) does it reach the feature drawn
// for itself. A value drawn nowhere, or one wiring two pins of one node, is no edge.
func (w *featureWalk) valueBindingEdges() {
	r, out := w.r, w.out
	for _, sym := range w.drawn {
		value := featureValue(sym)
		if value == nil {
			continue
		}
		from := w.siteOf(sym)
		target, resolved := r.resolver.ResolveTarget(sym.OwnerScope, value)
		if !resolved {
			continue
		}
		at := w.endNode(sym, value)
		if at.node == nil || at.node == from.node {
			at = w.siteOf(target)
		}
		if at.node == nil || at.node == from.node {
			continue
		}
		route, style := r.routeOf(w.view, sym, out), r.edgeDress(w.view, sym, from.node.ID, at.node.ID, out)
		out.Edges = append(out.Edges, Edge{
			From: from.node.ID, To: at.node.ID, FromPort: from.port, ToPort: at.port, Label: bindingKeyword, Kind: EdgeBinding,
			Origin: symbolOrigin(sym), Route: route, Style: style,
		})
	}
}

// bindingKeyword labels a binding edge no name of its own labels.
const bindingKeyword = "binding"

// featureValue is the feature a usage's value names — a feature reference or
// chain — nil for a usage with no value or one computed from an expression.
// A `default` value yields to any other and a `:=` value holds only at the
// start, so neither binds the feature and neither is one.
func featureValue(sym *symbols.Symbol) ast.Node {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil || usage.ValueIsDefault || usage.ValueIsInitial {
		return nil
	}
	switch usage.Value.(type) {
	case *ast.FeatureReference, *ast.FeatureChainExpr:
		return usage.Value
	}
	return nil
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
			feature := ast.EndTarget(end)
			out = append(out, connectorEnd{attachment: feature, path: lower.FeaturePath(feature)})
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

// edgeEnd is where a connector end attaches: a node, and the pin of it the end
// names, "" for the node itself.
type edgeEnd struct {
	node *Node
	port string
}

// endNode is where an end attaches: the feature it names when the rendering
// shows it, else the nearest feature owning it that the rendering shows — a
// port of a part the view exposes without exposing the port itself. A chain
// names a member of what its operand names: `control.durationOut` is the
// durationOut drawn under control's node, nested in it or pinned on its border
// — not the same port drawn for the type itself, or for another part of that
// type — and control's node where none is drawn. A bare name is first what the
// connector's owner draws for it — a port the owner has from its type, pinned on
// the owner's node, before the same port drawn nested under the type itself —
// and otherwise what the rendering draws for it anywhere.
func (w *featureWalk) endNode(connector *symbols.Symbol, attachment ast.Node) edgeEnd {
	if attachment == nil {
		return edgeEnd{}
	}
	target, resolved := w.r.resolver.ResolveTarget(connector.OwnerScope, attachment)
	if operand := chainOperand(attachment); operand != nil {
		if resolved && w.mixed && target.Owner() != nil && !mixedStructural(target.Owner()) {
			if node := w.nodes[target]; node != nil {
				return edgeEnd{node: node}
			}
		}
		if base := w.endNode(connector, operand); base.node != nil {
			if at := w.memberEnd(base.node, target); at.node != nil {
				return at
			}
			return base
		}
	}
	if !resolved {
		return edgeEnd{}
	}
	for owner := connector.Owner(); owner != nil; owner = owner.Owner() {
		if node, ok := w.nodes[owner]; ok {
			if at := w.memberEnd(node, target); at.node != nil {
				return at
			}
		}
	}
	for sym := target; sym != nil; sym = sym.Owner() {
		if node, ok := w.nodes[sym]; ok {
			return edgeEnd{node: node}
		}
	}
	return edgeEnd{}
}

// memberEnd is where a member of what base draws attaches: the node under base
// drawing it or the nearest feature owning it, or base's pin for it. The zero
// edgeEnd when base draws nothing for it.
func (w *featureWalk) memberEnd(base *Node, target *symbols.Symbol) edgeEnd {
	for sym := target; sym != nil; sym = sym.Owner() {
		if node, ok := w.nodes[sym]; ok && w.under(node, base) {
			return edgeEnd{node: node}
		}
		if pin, ok := w.pins[base][sym]; ok {
			return edgeEnd{node: base, port: pin}
		}
	}
	return edgeEnd{}
}

// under reports whether node is base or drawn nested in it.
func (w *featureWalk) under(node, base *Node) bool {
	for ; node != nil; node = w.parent[node] {
		if node == base {
			return true
		}
	}
	return false
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
// rendering shows as a node: the structural elements a system is built from, and
// the analysis cases bound to their values, which a parametric view draws beside
// them. Behaviors, views and namespaces are not, and are reported rather than drawn.
func featureLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolPortDef, symbols.SymbolPortUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage,
		symbols.SymbolAttributeDef, symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage,
		symbols.SymbolEnumerationDef, symbols.SymbolEnumerationUsage,
		symbols.SymbolInterfaceDef, symbols.SymbolConnectionDef, symbols.SymbolAllocationDef,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolUseCaseDef, symbols.SymbolUseCaseUsage,
		symbols.SymbolRequirementDef, symbols.SymbolRequirementUsage,
		symbols.SymbolKerMLType:
		return true
	}
	return false
}
