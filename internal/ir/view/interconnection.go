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
// nested in it is not a second root, nor is an exposed port a drawn feature
// pins on its border from its type.
func (r *Renderer) renderInterconnection(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	r.renderInterconnectionWithIDs(view, exposed, out, &nodeIDs{}, false)
}

// renderInterconnectionWithIDs walks structural features with shared node IDs.
func (r *Renderer) renderInterconnectionWithIDs(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering, ids *nodeIDs, mixed bool) map[*symbols.Symbol]*Node {
	w := &featureWalk{r: r, view: view, ids: ids, nodes: map[*symbols.Symbol][]*Node{}, pruned: map[*symbols.Symbol]bool{},
		pins: map[*Node]map[*symbols.Symbol]string{}, parent: map[*Node]*Node{}, out: out, mixed: mixed}
	var roots []*symbols.Symbol
	for _, elem := range exposed {
		if !r.drawsConnector(elem) && w.drawsFeature(elem) {
			roots = append(roots, elem)
		}
	}
	descendants := r.exposedDescendants(view, roots, w.nestedFeatures)
	var ports []*symbols.Symbol
	for _, elem := range exposed {
		switch {
		case r.drawsConnector(elem):
			w.connectors = append(w.connectors, connectorSite{sym: elem})
		case w.drawsFeature(elem):
			if descendants[symbols.KeyOf(elem)] {
				continue
			}
			if elem.Kind == symbols.SymbolPortUsage {
				ports = append(ports, elem)
				continue
			}
			out.Roots = append(out.Roots, w.featureNode(elem, expansion{}, 0, true))
		default:
			out.Notices = append(out.Notices, fmt.Sprintf(
				"%s %s has no place in an interconnection rendering; it is not shown",
				declKind(elem), r.notationName(elem)))
		}
	}
	w.exposedPorts(ports)
	w.connectorEdges()
	w.valueBindingEdges()
	first := make(map[*symbols.Symbol]*Node, len(w.nodes))
	for sym, nodes := range w.nodes {
		first[sym] = nodes[0]
	}
	return first
}

// exposedPorts marks each exposed port where the drawn usages of its owner pin
// it — on each of them, whatever the display shows — and draws the rest as nodes.
func (w *featureWalk) exposedPorts(ports []*symbols.Symbol) {
	for _, port := range ports {
		if w.node(port) != nil {
			continue
		}
		sites := w.pinSites(port)
		if len(sites) == 0 {
			w.out.Roots = append(w.out.Roots, w.featureNode(port, expansion{}, 0, true))
			continue
		}
		for _, site := range sites {
			for i := range site.node.Ports {
				if site.node.Ports[i].ID == site.port {
					site.node.Ports[i].Exposed = true
				}
			}
		}
	}
}

// connectorEdges draws each collected connector once: one found in a node is
// drawn there, and not again from outside when the view also exposes it.
func (w *featureWalk) connectorEdges() {
	seen, within := map[connectorSite]bool{}, map[*symbols.Symbol]bool{}
	for _, connector := range w.connectors {
		if connector.within != nil {
			within[connector.sym] = true
		}
	}
	for _, connector := range w.connectors {
		if seen[connector] || connector.within == nil && within[connector.sym] {
			continue
		}
		seen[connector] = true
		w.connectionEdges(connector.sym, connector.within)
	}
}

// featureWalk is one interconnection rendering's walk over the exposed features:
// the nodes rendered so far, by the feature each draws — a feature inherited
// into several usages is drawn under each — and the connectors collected along
// the way, each with the node it was found in. pins is the ports drawn on the
// border of a node rather than as nodes of their own — the ports a feature has
// from its type without declaring them, and a case's parameters — by node and
// port; sites is every feature drawn, as a node or a pin, in drawing order;
// parent is the node each nested node is drawn in.
type featureWalk struct {
	r          *Renderer
	view       *symbols.Symbol
	ids        *nodeIDs
	nodes      map[*symbols.Symbol][]*Node
	sites      []drawnSite
	pins       map[*Node]map[*symbols.Symbol]string
	parent     map[*Node]*Node
	connectors []connectorSite
	out        *Rendering
	mixed      bool
	// pruned is each member the view's filter conditions kept the walk from drawing.
	pruned map[*symbols.Symbol]bool
}

// drawnSite is one drawing of a feature: as a node, or as a pin of one.
type drawnSite struct {
	sym *symbols.Symbol
	at  edgeEnd
}

// connectorSite is a connector and the node whose members it was found among,
// nil for one the view exposes by itself; its bare end names resolve there first.
type connectorSite struct {
	sym    *symbols.Symbol
	within *Node
}

// node is the first node drawing sym, nil when none does.
func (w *featureWalk) node(sym *symbols.Symbol) *Node {
	if nodes := w.nodes[sym]; len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

// walkMembers is the renderer's walkMembers for the walk's view, remembering
// what the filters pruned so a connector ending there is reported, not rerouted.
func (w *featureWalk) walkMembers(sym *symbols.Symbol) []*symbols.Symbol {
	all := w.r.effectiveMembers(sym)
	kept := w.r.walkMembers(w.view, sym)
	if len(kept) == len(all) {
		return kept
	}
	drawn := map[*symbols.Symbol]bool{}
	for _, member := range kept {
		drawn[member] = true
	}
	for _, member := range all {
		if !drawn[member] {
			w.pruned[member] = true
		}
	}
	return kept
}

// excluded reports whether target, or an owner of it below the nearest drawn
// node, was pruned by the view's filter conditions: nothing stands in for it.
func (w *featureWalk) excluded(target *symbols.Symbol) bool {
	for sym := target; sym != nil; sym = sym.Owner() {
		if w.pruned[sym] {
			return true
		}
		if w.node(sym) != nil {
			return false
		}
	}
	return false
}

// pinnedAt reports whether member is drawn on owner's border rather than
// nested in it: a port, or a pin of a case or constraint.
func pinnedAt(owner, member *symbols.Symbol) bool {
	return member.Kind == symbols.SymbolPortUsage || casePin(owner, member) || constraintParameter(owner, member)
}

// drawsFeature reports whether the interconnection traversal emits sym as a feature node.
func (w *featureWalk) drawsFeature(sym *symbols.Symbol) bool {
	if !featureLike(sym) {
		return false
	}
	if !w.mixed {
		return true
	}
	return mixedStructural(sym)
}

// featureNode renders one exposed feature and the features nested in it,
// collecting the connectors declared along the way, which join the nodes it
// renders. A member it has from its type and pins is not drawn nested as well.
func (w *featureWalk) featureNode(sym *symbols.Symbol, path expansion, depth int, qualified bool) *Node {
	r := w.r
	name := r.notationName(sym)
	if !qualified {
		name = localName(sym)
	}
	node := &Node{ID: w.ids.take(), Kind: declKind(sym), Name: name, NameSynthesized: r.model.NameSynthesized(sym),
		Type: r.headType(sym), Typings: r.headTypings(sym), Origin: symbolOrigin(sym), Geometry: r.geometryOf(w.view, sym, w.out),
		Style: r.styleOf(w.view, sym, w.out)}
	if path[sym] {
		node.Detail = detailWith(node.Detail, "already shown as "+w.node(sym).ID)
		return node
	}
	w.nodes[sym] = append(w.nodes[sym], node)
	w.sites = append(w.sites, drawnSite{sym: sym, at: edgeEnd{node: node}})
	w.pinPorts(node, sym)
	w.pinParameters(node, sym)
	r.notesOf(w.view, sym, node.ID, w.out)
	recursive, added := r.enter(path, sym)
	defer leave(path, added)
	if recursive != nil {
		node.Detail = detailWith(node.Detail, recursionNote(recursive))
	}
	// At the depth bound, or collapsed, an element keeps its ports and pins — they
	// count at their owner's depth — and nothing nested is drawn.
	bound, cut := r.walkBound(w.view), 0
	folded := depth >= bound || collapsed(node)
	for _, member := range w.walkMembers(sym) {
		if _, pinned := w.pins[node][member]; pinned || recursive != nil && member.Owner() != sym {
			continue
		}
		if folded && !pinnedAt(sym, member) {
			if depth >= bound && w.drawsFeature(member) {
				cut++
			}
			continue
		}
		switch {
		case r.drawsConnector(member):
			w.connectors = append(w.connectors, connectorSite{sym: member, within: node})
		case casePin(sym, member):
			w.pin(node, member, pinDirection(member), true, r.geometryOf(w.view, member, w.out))
		case constraintParameter(sym, member):
			w.pin(node, member, pinDirection(member), false, r.geometryOf(w.view, member, w.out))
		case isReferencedIncludedCase(member):
			// an `include u;` is drawn by the connection to the case it names
		case w.drawsFeature(member):
			child := w.featureNode(member, path, depth+1, false)
			w.parent[child] = node
			node.Children = append(node.Children, child)
		}
	}
	if cut > 0 {
		node.Detail = detailWith(node.Detail, cutNote(bound))
		w.out.depthCut += cut
	}
	return node
}

// pinPorts draws on node's border the ports sym has from its type, each named
// and typed as the type declares it: `heating : HeatingSystem` shows the
// `durationIn : ~DurationPort` HeatingSystem declares, and a connector ending
// at `heating.durationIn` ends at that pin.
func (w *featureWalk) pinPorts(node *Node, sym *symbols.Symbol) {
	for _, port := range w.r.typedPorts(sym) {
		if !w.admitted(port) {
			continue
		}
		w.pin(node, port, PortUndirected, false, w.r.memberGeometryOf(w.view, sym, localName(port), w.out))
	}
}

// admitted reports whether the view's filter conditions admit an inherited port
// or parameter sym pins; one they reject is pruned like a declared member.
func (w *featureWalk) admitted(sym *symbols.Symbol) bool {
	if w.r.admits(w.r.walkFilters(w.view), sym) {
		return true
	}
	if w.pruned == nil {
		w.pruned = map[*symbols.Symbol]bool{}
	}
	w.pruned[sym] = true
	return false
}

// pinParameters draws on node's border the parameters sym has from its constraint
// def without declaring them: `constraint c : Law` shows the `in x` Law declares.
func (w *featureWalk) pinParameters(node *Node, sym *symbols.Symbol) {
	for _, parameter := range w.r.typedParameters(sym) {
		if !w.admitted(parameter) {
			continue
		}
		w.pin(node, parameter, pinDirection(parameter), false, w.r.memberGeometryOf(w.view, sym, localName(parameter), w.out))
	}
}

// pin draws sym as a port on node's border, with the given direction and the geometry
// a Layout gives it; a parameter pin stands for a case's subject or attribute, not a port.
func (w *featureWalk) pin(node *Node, sym *symbols.Symbol, direction PortDirection, parameter bool, geometry *Geometry) {
	id := portID(node.ID, len(node.Ports))
	node.Ports = append(node.Ports, Port{ID: id, Name: localName(sym), NameSynthesized: w.r.model.NameSynthesized(sym),
		Type: w.r.headType(sym), Direction: direction, Parameter: parameter, Origin: symbolOrigin(sym), Geometry: geometry})
	if w.pins[node] == nil {
		w.pins[node] = map[*symbols.Symbol]string{}
	}
	w.pins[node][sym] = id
	w.sites = append(w.sites, drawnSite{sym: sym, at: edgeEnd{node: node, port: id}})
}

// siteOf is where a drawn feature is: its own node, or the node it is a pin of
// and the pin; the zero edgeEnd for one not drawn.
func (w *featureWalk) siteOf(sym *symbols.Symbol) edgeEnd {
	if node := w.node(sym); node != nil {
		return edgeEnd{node: node}
	}
	return w.pinSite(sym)
}

// pinSite is the node that pins sym on its border, and the pin, when exactly
// one drawn node does; the zero edgeEnd when none or several do, since a bare
// name of the port names no one of them.
func (w *featureWalk) pinSite(sym *symbols.Symbol) edgeEnd {
	if sites := w.pinSites(sym); len(sites) == 1 {
		return sites[0]
	}
	return edgeEnd{}
}

// pinSites is each node that pins sym on its border, with the pin, in drawing order.
func (w *featureWalk) pinSites(sym *symbols.Symbol) []edgeEnd {
	var sites []edgeEnd
	for _, drawn := range w.sites {
		if drawn.sym == sym && drawn.at.port != "" {
			sites = append(sites, drawn.at)
		}
	}
	return sites
}

// casePin reports whether member, a member of a case drawn as a node, is
// drawn as a pin on its border rather than a node inside it: the case's
// subject, actors, parameters and attributes, as the graphical notation draws
// a case's parameters. Its parts, actions and nested cases are not.
func casePin(owner, member *symbols.Symbol) bool {
	switch owner.Kind {
	case symbols.SymbolCaseDef, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseDef, symbols.SymbolUseCaseUsage:
	default:
		return false
	}
	switch member.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage:
		return true
	}
	usage, ok := member.Decl.(*ast.Usage)
	return ok && (usage.Kind == ast.UsageSubject || usage.Kind == ast.UsageActor || usage.Kind == ast.UsageStakeholder)
}

// constraintParameter reports whether member is a parameter of the constraint owner —
// a directed attribute or reference usage — drawn as a square on the constraint's border.
func constraintParameter(owner, member *symbols.Symbol) bool {
	return isConstraintKind(owner) && isParameterKind(member) && pinDirection(member) != PortUndirected
}

// isConstraintKind reports whether sym is a constraint def or usage.
func isConstraintKind(sym *symbols.Symbol) bool {
	return sym.Kind == symbols.SymbolConstraintDef || sym.Kind == symbols.SymbolConstraintUsage
}

// isParameterKind reports whether sym is a usage a constraint's parameter is declared as.
func isParameterKind(sym *symbols.Symbol) bool {
	return sym.Kind == symbols.SymbolAttributeUsage || sym.Kind == symbols.SymbolReferenceUsage
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

// typedParameters is the parameters a constraint has from its def and what that specializes
// without declaring them, less any it redefines and the library's own features of every constraint.
func (r *Renderer) typedParameters(sym *symbols.Symbol) []*symbols.Symbol {
	if !isConstraintKind(sym) {
		return nil
	}
	owned := map[*symbols.Symbol]bool{}
	for _, member := range r.containedMembers(sym) {
		owned[member] = true
	}
	var out []*symbols.Symbol
	for _, member := range r.model.MembersOf(sym) {
		if !isParameterKind(member) || pinDirection(member) == PortUndirected || owned[member] || member.Owner() == sym ||
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
	if idx == nil {
		return false
	}
	// The index marks what it names; an unnamed member is the library's when
	// what owns it is.
	for ; sym != nil; sym = sym.Owner() {
		if idx.Library(sym) {
			return true
		}
	}
	return false
}

// nestedFeatures is what featureNode draws under sym: its nested feature nodes and the
// parameters pinned from its def, which a view exposing them beside it does not draw again.
// A port it has from its type is a pin, which an exposure of the port marks; a pin of a
// case or constraint shows nothing nested in it.
func (w *featureWalk) nestedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if owner := sym.Owner(); owner != nil && (casePin(owner, sym) || constraintParameter(owner, sym)) {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range w.walkMembers(sym) {
		if member.Kind == symbols.SymbolPortUsage && member.Owner() != sym {
			continue
		}
		if w.drawsFeature(member) && !isReferencedIncludedCase(member) {
			out = append(out, member)
		}
	}
	return append(out, w.r.typedParameters(sym)...)
}

// connectionEdges adds the edges one connector or flow contributes. A binary
// connector joins its two ends; a multi-end connector makes every end reachable
// from every other, so each pair is an edge. An end attaching to something the
// view does not expose is reported rather than dropped. within is the node the
// connector was found in, where its ends are looked for first.
func (w *featureWalk) connectionEdges(connector *symbols.Symbol, within *Node) {
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
		at := w.endNode(connector, within, end.attachment)
		if at.node == nil {
			why := "which the view does not expose"
			if target, ok := r.resolver.ResolveTarget(connector.OwnerScope, end.attachment); ok && w.excluded(target) {
				why = "which the view's filter conditions exclude"
			}
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s attaches to %s, %s; no edge is drawn",
				declKind(connector), r.notationName(connector), notationName(end.path), why))
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
// for itself. A feature drawn in several places binds from each. A value drawn
// nowhere, or one wiring two pins of one node, is no edge.
func (w *featureWalk) valueBindingEdges() {
	r, out := w.r, w.out
	for _, drawn := range w.sites {
		sym, from := drawn.sym, drawn.at
		value := featureValue(sym)
		if value == nil {
			continue
		}
		target, resolved := r.resolver.ResolveTarget(sym.OwnerScope, value)
		if !resolved {
			continue
		}
		within := from.node
		if from.port == "" {
			within = w.parent[from.node]
		}
		at := w.endNode(sym, within, value)
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

// bindingKeyword labels a binding edge no name of its own labels: the `=` of
// the graphical notation's binding connection.
const bindingKeyword = "="

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
// and otherwise what the rendering draws for it anywhere. within is the node
// the connector was found in — the usage whose type declares it, when it is
// inherited — and what it draws for a name comes before what its owner draws.
func (w *featureWalk) endNode(connector *symbols.Symbol, within *Node, attachment ast.Node) edgeEnd {
	if attachment == nil {
		return edgeEnd{}
	}
	target, resolved := w.r.resolver.ResolveTarget(connector.OwnerScope, attachment)
	if operand := chainOperand(attachment); operand != nil {
		if at, ok := w.chainEnd(connector, within, operand, target, resolved); ok {
			return at
		}
	}
	if !resolved || w.excluded(target) {
		return edgeEnd{}
	}
	if within != nil {
		if at := w.memberEnd(within, target); at.node != nil {
			return at
		}
	}
	for owner := connector.Owner(); owner != nil; owner = owner.Owner() {
		for _, node := range w.nodes[owner] {
			if at := w.memberEnd(node, target); at.node != nil {
				return at
			}
		}
	}
	for sym := target; sym != nil; sym = sym.Owner() {
		if node := w.node(sym); node != nil {
			return edgeEnd{node: node}
		}
	}
	// A port of a definition drawn only as the pin of the one usage typed by it.
	return w.pinSite(target)
}

// chainEnd is where a feature chain's target attaches under what its operand
// draws; ok is false when the operand draws nothing, leaving the end to the
// rendering at large.
func (w *featureWalk) chainEnd(connector *symbols.Symbol, within *Node, operand ast.Node, target *symbols.Symbol, resolved bool) (at edgeEnd, ok bool) {
	if resolved && w.mixed && target.Owner() != nil && !mixedStructural(target.Owner()) {
		if node := w.node(target); node != nil {
			return edgeEnd{node: node}, true
		}
	}
	base := w.endNode(connector, within, operand)
	if base.node == nil {
		return edgeEnd{}, false
	}
	if at := w.memberEnd(base.node, target); at.node != nil {
		return at, true
	}
	// A member the view's filters exclude ends no edge at its owner; one
	// below the depth bound or a collapsed body does.
	if resolved && w.excluded(target) {
		return edgeEnd{}, true
	}
	return base, true
}

// memberEnd is where a member of what base draws attaches: the node under base
// drawing it — or drawing the feature that redefines it there, which stands in
// its place — or the nearest feature owning it, or base's pin for it. The zero
// edgeEnd when base draws nothing for it.
func (w *featureWalk) memberEnd(base *Node, target *symbols.Symbol) edgeEnd {
	if w.excluded(target) {
		return edgeEnd{}
	}
	for sym := target; sym != nil; sym = sym.Owner() {
		for _, node := range w.nodes[sym] {
			if w.under(node, base) {
				return edgeEnd{node: node}
			}
		}
		if pin, ok := w.pins[base][sym]; ok {
			return edgeEnd{node: base, port: pin}
		}
		if at := w.redefiningEnd(base, sym); at.node != nil {
			return at
		}
	}
	return edgeEnd{}
}

// redefiningEnd is where a feature redefining sym is drawn as base's own: a
// node nested directly in base or a pin of it, not one of a part nested
// deeper, which redefines the feature for itself. `:>> engines = (e1, e2)` is
// the engines a binding to engines names.
func (w *featureWalk) redefiningEnd(base *Node, sym *symbols.Symbol) edgeEnd {
	for _, drawn := range w.sites {
		if drawn.sym == sym || drawn.at.node == nil {
			continue
		}
		if drawn.at.port == "" && w.parent[drawn.at.node] != base || drawn.at.port != "" && drawn.at.node != base {
			continue
		}
		if slices.Contains(w.r.model.RedefinedFeatures(drawn.sym), sym) {
			return drawn.at
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

// featureLike reports whether an interconnection rendering shows sym as a node: structures and the
// constraints and analysis cases bound to their values; behaviors, views and namespaces are reported.
func featureLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage,
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
