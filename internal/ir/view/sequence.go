package view

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func runParticipantLabel(node *Node) string {
	label := node.Name
	if node.Type != "" {
		if label != "" {
			label += " : "
		}
		label += node.Type
	}
	if label == "" {
		return node.Kind
	}
	return label
}

// renderSequence renders the occurrences of an interaction as lifelines and the
// flows between them as directed messages, ordered as the model states. A
// lifeline list is flat — a sequence diagram nests nothing — so a message
// attaching to an event nested in a lifeline is drawn on that lifeline. Anything
// a sequence rendering cannot show is reported rather than dropped.
func (r *Renderer) renderSequence(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ends := &lifelineEnds{r: r, view: view, out: out, ids: &nodeIDs{},
		lifelines: map[*symbols.Symbol]*Node{}, proxies: map[lifelineEnd]*Node{}, ports: map[lifelineEnd]string{}}
	stated := &interactions{}
	visited := map[*symbols.Symbol]bool{}
	var behaviors []*symbols.Symbol
	for _, elem := range exposed {
		switch {
		case isFlowUsage(elem):
			stated.addFlow(elem)
		case isSuccessionUsage(elem):
			r.addSuccessionUsage(elem, stated)
		case r.model.IsConnectorUsage(elem):
			stated.addConnector(elem)
		case behaviorLike(elem):
			if ends.lifelines[elem] == nil {
				ends.lifelines[elem] = r.lifeline(view, elem, elem, ends.ids, out)
				out.Roots = append(out.Roots, ends.lifelines[elem])
				behaviors = append(behaviors, elem)
				r.addEventOrders(elem, stated)
			}
		case occurrenceContainer(elem), lifelineLike(elem):
			for _, participant := range r.lifelinesOf(elem) {
				if ends.lifelines[participant] != nil {
					continue
				}
				node := r.lifeline(view, elem, participant, ends.ids, out)
				out.Roots = append(out.Roots, node)
				ends.lifelines[participant] = node
			}
			r.collectInteractions(elem, stated, visited, 0)
		default:
			out.Notices = append(out.Notices, fmt.Sprintf(
				"%s %s has no place in a sequence rendering; it is not shown",
				declKind(elem), r.notationName(elem)))
		}
	}
	out.Notices = append(out.Notices, stated.notices...)
	for _, connector := range stated.connectors {
		out.Notices = append(out.Notices, fmt.Sprintf(
			"%s states no direction; a sequence rendering shows directed messages only",
			r.noticeSubject(connector)))
	}
	messages := r.messages(view, stated.flows, ends, out)
	successions, timings := r.successions(view, stated.orders, ends, out)
	out.Edges = r.orderMessages(messages, successions, timings, out)
	r.dropIdleBehaviors(behaviors, ends, out)
	ends.sortProxies()
}

// lifeline is the node drawn for a participant of the exposed element elem, named
// in full when it is that element itself.
func (r *Renderer) lifeline(view, elem, participant *symbols.Symbol, ids *nodeIDs, out *Rendering) *Node {
	name := r.notationName(participant)
	if participant != elem {
		name = localName(participant)
	}
	node := &Node{ID: ids.take(), Kind: declKind(participant), Name: name, NameSynthesized: r.model.NameSynthesized(participant),
		Type: r.headType(participant), Typings: r.headTypings(participant), Origin: symbolOrigin(participant),
		Geometry: r.geometryOf(view, participant, out)}
	r.dress(view, participant, node, out)
	return node
}

// addEventOrders records the orders an exposed behavior states between the events
// it holds; a succession reaching one of its actions is its control flow, not one.
func (r *Renderer) addEventOrders(behavior *symbols.Symbol, into *interactions) {
	stated := len(into.orders)
	r.addSuccessionEdges(behavior, into)
	for _, member := range r.containedMembers(behavior) {
		if isSuccessionUsage(member) {
			r.addSuccessionUsage(member, into)
		}
	}
	kept := into.orders[:stated]
	for _, o := range into.orders[stated:] {
		if o.from != behavior && o.to != behavior && !behaviorLike(o.from) && !behaviorLike(o.to) {
			kept = append(kept, o)
		} else if o.succession != nil {
			// Left for a view that exposes the succession itself.
			delete(into.seen, o.succession)
		}
	}
	into.orders = kept
}

// dropIdleBehaviors takes the lifeline of each exposed behavior no message or order
// reaches out of the rendering and reports it: only its action flow was there to show.
func (r *Renderer) dropIdleBehaviors(behaviors []*symbols.Symbol, ends *lifelineEnds, out *Rendering) {
	reached := map[string]bool{}
	for _, edge := range out.Edges {
		reached[edge.From], reached[edge.To] = true, true
	}
	for _, behavior := range behaviors {
		node := ends.lifelines[behavior]
		idle := !reached[node.ID]
		for _, proxy := range node.Children {
			idle = idle && !reached[proxy.ID]
		}
		if !idle {
			continue
		}
		delete(ends.lifelines, behavior)
		out.Roots = slices.DeleteFunc(out.Roots, func(root *Node) bool { return root == node })
		gone := map[string]bool{node.ID: true}
		for _, proxy := range node.Children {
			gone[proxy.ID] = true
		}
		out.Notes = slices.DeleteFunc(out.Notes, func(note Note) bool { return gone[note.Anchor] })
		out.Notices = append(out.Notices, fmt.Sprintf(
			"%s is behavior, not a participant in an interaction; a sequence rendering does not show it",
			r.noticeSubject(behavior)))
	}
}

// lifelineEnds are the places a message or succession ends on the lifelines: the
// lifelines themselves, the proxies of the events a lifeline's body declares (SysML
// v2.1 §8.2.3.9 sq-proxy), and the ports on a lifeline's border (sq-port).
type lifelineEnds struct {
	r         *Renderer
	view      *symbols.Symbol
	out       *Rendering
	ids       *nodeIDs
	lifelines map[*symbols.Symbol]*Node
	proxies   map[lifelineEnd]*Node
	ports     map[lifelineEnd]string
}

// lifelineEnd is an occurrence on one lifeline: lifelines typed alike each hold
// the events their definition declares.
type lifelineEnd struct {
	lifeline   *Node
	occurrence *symbols.Symbol
}

// before orders two ends by where their occurrences are declared, then by their
// lifelines as drawn, an end on no lifeline first.
func (e lifelineEnd) before(other lifelineEnd) bool {
	if e.occurrence != other.occurrence {
		return declaredBefore(e.occurrence, other.occurrence)
	}
	if e.lifeline == nil || other.lifeline == nil {
		return e.lifeline == nil && other.lifeline != nil
	}
	// IDs are taken in drawing order: n2 before n10.
	if a, b := e.lifeline.ID, other.lifeline.ID; len(a) != len(b) {
		return len(a) < len(b)
	}
	return e.lifeline.ID < other.lifeline.ID
}

// sortProxies puts each lifeline's proxies in the order their events are declared,
// whatever order the messages reached them in.
func (e *lifelineEnds) sortProxies() {
	declared := make(map[*Node]*symbols.Symbol, len(e.proxies))
	for end, proxy := range e.proxies {
		declared[proxy] = end.occurrence
	}
	for _, lifeline := range e.lifelines {
		sort.SliceStable(lifeline.Children, func(i, j int) bool {
			return declaredBefore(declared[lifeline.Children[i]], declared[lifeline.Children[j]])
		})
	}
}

// lifelineOf is the lifeline an occurrence is on: the nearest of its owners drawn as one.
func (e *lifelineEnds) lifelineOf(occurrence *symbols.Symbol) *Node {
	for sym := occurrence; sym != nil; sym = sym.Owner() {
		if node, ok := e.lifelines[sym]; ok {
			return node
		}
	}
	return nil
}

// end is where an edge to occurrence is drawn on its lifeline: at the lifeline
// itself, at the proxy of an event it holds, or at one of its ports.
func (e *lifelineEnds) end(occurrence *symbols.Symbol, lifeline *Node) (node, port string) {
	if e.lifelines[occurrence] == lifeline {
		return lifeline.ID, ""
	}
	if isPortUsage(occurrence) {
		id, ok := e.ports[lifelineEnd{lifeline, occurrence}]
		if !ok {
			id = e.ids.take()
			e.ports[lifelineEnd{lifeline, occurrence}] = id
			lifeline.Ports = append(lifeline.Ports, Port{ID: id, Name: localName(occurrence), Type: declType(occurrence),
				Direction: PortUndirected, Origin: symbolOrigin(occurrence)})
		}
		return lifeline.ID, id
	}
	if !lifelineLike(occurrence) {
		return lifeline.ID, ""
	}
	proxy, ok := e.proxies[lifelineEnd{lifeline, occurrence}]
	if !ok {
		// A proxy's place is its row on the lifeline, so no Layout positions it; its Style and Notes apply.
		proxy = &Node{ID: e.ids.take(), Kind: declKind(occurrence), Name: localName(occurrence), Origin: symbolOrigin(occurrence)}
		e.r.dress(e.view, occurrence, proxy, e.out)
		e.proxies[lifelineEnd{lifeline, occurrence}] = proxy
		lifeline.Children = append(lifeline.Children, proxy)
	}
	return proxy.ID, ""
}

// timed reports an occurrence drawn as a proxy on lifeline: an event of its own.
func (e *lifelineEnds) timed(occurrence *symbols.Symbol, lifeline *Node) bool {
	_, ok := e.proxies[lifelineEnd{lifeline, occurrence}]
	return ok
}

// isPortUsage reports a port usage, which a sequence draws on its lifeline's border.
func isPortUsage(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolPortUsage
}

// lifelinesOf are the lifelines an exposed element contributes: the occurrences
// an interaction declares in its body, else the element itself when it declares
// none. The container is not a lifeline of its own interaction, and neither is
// behavior it performs, which a sequence rendering shows as messages instead.
func (r *Renderer) lifelinesOf(elem *symbols.Symbol) []*symbols.Symbol {
	if occurrenceContainer(elem) {
		var participants []*symbols.Symbol
		for _, member := range r.containedMembers(elem) {
			if lifelineLike(member) && !behaviorLike(member) {
				participants = append(participants, member)
			}
		}
		if len(participants) > 0 {
			return participants
		}
	}
	return []*symbols.Symbol{elem}
}

// message is one message before its order is settled: the edge to draw, the flow
// usage that stated it, and the occurrences its ends name, which the order is
// read over.
type message struct {
	flow     *symbols.Symbol
	from, to lifelineEnd
	// Whether each end is an event of its own on the lifeline, with a time the
	// orders can state, rather than the lifeline itself or a port of it.
	fromTimed, toTimed bool
	edge               Edge
}

// order is a sequence the model states between two occurrences, and how a notice
// names it.
type order struct {
	from, to *symbols.Symbol
	// The features a succession usage names its ends through (`client` of
	// `client.sent`), nil for an end named on its own.
	fromThrough, toThrough *symbols.Symbol
	// within is the element whose body states the order.
	within *symbols.Symbol
	name   string
	// succession is the usage stating the order, nil for a member-attached `then`,
	// which has no declaration of its own.
	succession *symbols.Symbol
	origin     Origin
}

// interactions are what a sequence rendering reads between lifelines: the flows
// that are messages, the orders stated over the occurrences they reach, and the
// undirected connectors that cannot be messages. Each is recorded once.
type interactions struct {
	flows      []*symbols.Symbol
	connectors []*symbols.Symbol
	orders     []order
	notices    []string
	seen       map[*symbols.Symbol]bool
}

// first reports whether a usage is being recorded for the first time.
func (i *interactions) first(sym *symbols.Symbol) bool {
	if i.seen == nil {
		i.seen = map[*symbols.Symbol]bool{}
	}
	if i.seen[sym] {
		return false
	}
	i.seen[sym] = true
	return true
}

func (i *interactions) addFlow(sym *symbols.Symbol) {
	if i.first(sym) {
		i.flows = append(i.flows, sym)
	}
}

func (i *interactions) addConnector(sym *symbols.Symbol) {
	if i.first(sym) {
		i.connectors = append(i.connectors, sym)
	}
}

// collectInteractions gathers what is stated within an element and within the
// members nested in it, which is where a message and the order over it are
// written.
func (r *Renderer) collectInteractions(sym *symbols.Symbol, into *interactions, visited map[*symbols.Symbol]bool, depth int) {
	if sym == nil || depth >= maxTreeDepth || visited[sym] {
		return
	}
	visited[sym] = true
	r.addSuccessionEdges(sym, into)
	for _, member := range r.containedMembers(sym) {
		switch {
		case isFlowUsage(member):
			into.addFlow(member)
		case isSuccessionUsage(member):
			r.addSuccessionUsage(member, into)
		case r.model.IsConnectorUsage(member):
			into.addConnector(member)
		case behaviorLike(member):
			if !r.statesFlow(member, depth+1) {
				continue
			}
			into.notices = append(into.notices, fmt.Sprintf(
				"the flows in %s are its action flow, not interaction messages; they are not shown",
				r.noticeSubject(member)))
		default:
			r.collectInteractions(member, into, visited, depth+1)
		}
	}
}

// statesFlow reports whether an element or anything nested in it states a flow,
// which is what there is to report rather than draw as a message.
func (r *Renderer) statesFlow(sym *symbols.Symbol, depth int) bool {
	if sym == nil || depth >= maxTreeDepth {
		return false
	}
	for _, member := range r.containedMembers(sym) {
		if isFlowUsage(member) || r.statesFlow(member, depth+1) {
			return true
		}
	}
	return false
}

// addSuccessionUsage records the order a `succession first a then b;` states,
// read from the model's own connector ends. An end naming nothing states no
// order, which is ordinary structure rather than a failure.
func (r *Renderer) addSuccessionUsage(succession *symbols.Symbol, into *interactions) {
	usage, _ := succession.Decl.(*ast.Usage)
	if usage == nil || len(usage.ConnectorEnds) != 2 || !into.first(succession) {
		return
	}
	from, fromOK := r.resolver.ResolveTarget(succession.OwnerScope, usage.ConnectorEnds[0].AttachedTarget())
	to, toOK := r.resolver.ResolveTarget(succession.OwnerScope, usage.ConnectorEnds[1].AttachedTarget())
	if !fromOK || !toOK {
		return
	}
	name := ""
	if succession.Named() && !succession.EffectiveName() {
		name = localName(succession)
	}
	fromThrough, _ := r.resolver.ResolveTarget(succession.OwnerScope, chainOperand(usage.ConnectorEnds[0].AttachedTarget()))
	toThrough, _ := r.resolver.ResolveTarget(succession.OwnerScope, chainOperand(usage.ConnectorEnds[1].AttachedTarget()))
	into.orders = append(into.orders, order{from: from, to: to, fromThrough: fromThrough, toThrough: toThrough, within: succession.Owner(),
		name: name, succession: succession, origin: symbolOrigin(succession)})
}

// addSuccessionEdges records the orders a member-attached `then` states, which
// the parser desugars into the same succession edges `first a then b` builds.
func (r *Renderer) addSuccessionEdges(sym *symbols.Symbol, into *interactions) {
	for _, member := range declaredMembers(sym.Decl) {
		edge, ok := member.(*ast.SuccessionEdge)
		if !ok {
			continue
		}
		from := r.edgeEnd(sym, edge.Source, edge.SourceMember)
		to := r.edgeEnd(sym, edge.Target, edge.TargetMember)
		if from == nil || to == nil {
			continue
		}
		into.orders = append(into.orders, order{from: from, to: to, within: sym, origin: nodeOrigin(symbols.DocNameOf(sym.OwnerScope), edge)})
	}
}

// edgeEnd is the occurrence an end of a succession edge names, or the member it
// is bound to by position when the notation left it unnamed.
func (r *Renderer) edgeEnd(owner *symbols.Symbol, name *ast.QualifiedName, member ast.Node) *symbols.Symbol {
	if name != nil && len(name.Parts) > 0 {
		if sym, ok := r.resolver.ResolveTarget(owner.Scope, name); ok {
			return sym
		}
		return nil
	}
	for _, candidate := range r.containedMembers(owner) {
		if member != nil && candidate.Decl == member {
			return candidate
		}
	}
	return nil
}

// declaredMembers are the body members of a declaration, as written: the edges
// between members are among them, and they are no symbols of their own.
func declaredMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		return nil
	}
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if ms, ok := member.(*ast.Membership); ok && ms.Member != nil {
			member = ms.Member
		}
		out = append(out, member)
	}
	return out
}

// messages is the message each flow contributes, in the order a reader reads the
// declarations. An end naming nothing the rendering shows is reported and the
// message dropped; both ends resolving to one lifeline is a self-message, which
// a sequence diagram draws.
func (r *Renderer) messages(view *symbols.Symbol, flows []*symbols.Symbol, ends *lifelineEnds, out *Rendering) []message {
	var messages []message
	for _, flow := range flows {
		usage, _ := flow.Decl.(*ast.Usage)
		if usage == nil || usage.FlowEnds == nil || usage.FlowEnds.From == nil || usage.FlowEnds.To == nil {
			out.Notices = append(out.Notices, fmt.Sprintf("%s states no source and target; no message is drawn",
				r.subject(flow, messageKind(flow))))
			continue
		}
		var occurrences []*symbols.Symbol
		var lifelines []*Node
		var nodes, ports []string
		for _, end := range []ast.Node{usage.FlowEnds.From, usage.FlowEnds.To} {
			occurrence, lifeline := r.messageEnd(flow, ast.EndTarget(end), ends)
			if lifeline == nil {
				out.Notices = append(out.Notices, fmt.Sprintf(
					"%s attaches to %s, which is on no lifeline the rendering shows; no message is drawn",
					r.subject(flow, messageKind(flow)), endText(end)))
				break
			}
			node, port := ends.end(occurrence, lifeline)
			occurrences, lifelines = append(occurrences, occurrence), append(lifelines, lifeline)
			nodes, ports = append(nodes, node), append(ports, port)
		}
		if len(nodes) != 2 {
			continue
		}
		kind := EdgeFlow
		if messageKind(flow) == "message" {
			kind = EdgeMessage
		}
		messages = append(messages, message{
			flow: flow, from: lifelineEnd{lifelines[0], occurrences[0]}, to: lifelineEnd{lifelines[1], occurrences[1]}, fromTimed: ends.timed(occurrences[0], lifelines[0]), toTimed: ends.timed(occurrences[1], lifelines[1]),
			edge: Edge{From: nodes[0], To: nodes[1], FromPort: ports[0], ToPort: ports[1], Label: r.messageLabel(flow), Kind: kind,
				Origin: symbolOrigin(flow), Route: r.routeOf(view, flow, out),
				Style: r.edgeDress(view, flow, nodes[0], nodes[1], out)},
		})
	}
	sort.SliceStable(messages, func(i, j int) bool { return declaredBefore(messages[i].flow, messages[j].flow) })
	return messages
}

// messageEnd is the occurrence an end of a message names and the lifeline the
// arrow is drawn on: the whole feature chain first, then a segment shorter, then
// the nearest owner shown — `producer.publish_source_event` is an event of the
// `producer` lifeline. An event the chain reaches through a lifeline that inherits
// it is an event of that lifeline, not of the one declaring it.
func (r *Renderer) messageEnd(flow *symbols.Symbol, end ast.Node, ends *lifelineEnds) (*symbols.Symbol, *Node) {
	if named, ok := r.resolver.ResolveTarget(flow.OwnerScope, end); ok {
		through, _ := r.resolver.ResolveTarget(flow.OwnerScope, chainOperand(end))
		if lifeline := ends.inheritedOn(named, through); lifeline != nil {
			return named, lifeline
		}
	}
	for attachment := end; attachment != nil; attachment = chainOperand(attachment) {
		target, ok := r.resolver.ResolveTarget(flow.OwnerScope, attachment)
		if !ok {
			continue
		}
		if lifeline := ends.lifelineOf(target); lifeline != nil {
			return target, lifeline
		}
	}
	return nil, nil
}

// successions are the orders drawn as the dashed succession arrows of the sequence
// (SysML v2.1 §8.2.3.9 sq-succession), each between the proxies or lifelines of
// its ends; an order with an end on no lifeline shown is kept for sorting only.
func (r *Renderer) successions(view *symbols.Symbol, orders []order, ends *lifelineEnds, out *Rendering) ([]succession, []timing) {
	var successions []succession
	var timings []timing
	inherited := r.inheritedOrders(orders, ends, out.Roots)
	for i, o := range orders {
		placed := []timing{{from: ends.orderEnd(o, o.from, o.fromThrough), to: ends.orderEnd(o, o.to, o.toThrough), name: o.name}}
		for _, t := range append(placed, inherited[i]...) {
			timings = append(timings, t)
			if t.from.lifeline == nil || t.to.lifeline == nil {
				continue
			}
			fromNode, fromPort := ends.end(t.from.occurrence, t.from.lifeline)
			toNode, toPort := ends.end(t.to.occurrence, t.to.lifeline)
			edge := Edge{From: fromNode, To: toNode, FromPort: fromPort, ToPort: toPort, Label: o.name, Name: o.name, Kind: EdgeSuccession, Origin: o.origin}
			if o.succession != nil {
				edge.Route = r.routeOf(view, o.succession, out)
				edge.Style = r.edgeDress(view, o.succession, fromNode, toNode, out)
			}
			successions = append(successions, succession{order: len(timings) - 1, edge: edge})
		}
	}
	return successions, timings
}

// inheritedOn is the lifeline an occurrence is on by inheritance: the one of the
// feature it is named through or stated within, when that is shown and inherits it.
func (e *lifelineEnds) inheritedOn(occurrence, through *symbols.Symbol) *Node {
	if through == nil || occurrence == nil || occurrence.Owner() == through || e.lifelines[through] == nil ||
		e.r.memberFor(through, occurrence) != occurrence {
		return nil
	}
	return e.lifelines[through]
}

// memberFor is the feature that stands for one in owner: the feature itself where
// owner declares or inherits it, the member redefining it where it is masked, nil
// where owner neither declares nor inherits it. An unnamed feature, which no member
// list holds, stands for itself.
func (r *Renderer) memberFor(owner, feature *symbols.Symbol) *symbols.Symbol {
	if owner == nil || feature == nil || feature.Owner() != owner && !r.specializes(owner, feature.Owner()) {
		return nil
	}
	members := r.model.MembersOf(owner)
	same := func(sym *symbols.Symbol) bool { return symbols.SameElement(sym, feature) }
	if slices.ContainsFunc(members, same) {
		return feature
	}
	for _, member := range members {
		if slices.ContainsFunc(r.model.AllRedefinedFeatures(member), same) {
			return member
		}
	}
	return feature
}

// specializes reports whether general is among what sym specializes or is typed by.
func (r *Renderer) specializes(sym, general *symbols.Symbol) bool {
	return general != nil && slices.ContainsFunc(r.model.AllSupertypes(sym), func(super *symbols.Symbol) bool {
		return symbols.SameElement(super, general)
	})
}

// orderEnd is an end of an order on the lifeline it is read on: the one it is
// named through, or stated within, when that inherits it, else the nearest owner shown.
func (e *lifelineEnds) orderEnd(o order, occurrence, through *symbols.Symbol) lifelineEnd {
	if through == nil {
		through = o.within
	}
	if lifeline := e.inheritedOn(occurrence, through); lifeline != nil {
		return lifelineEnd{lifeline, occurrence}
	}
	return lifelineEnd{e.lifelineOf(occurrence), occurrence}
}

// inheritedOrders are, per order, its readings on the lifelines that inherit it,
// in the order drawn: an order a type states between two events of its own or
// inherited holds on every lifeline specializing it that holds one of them, the
// events it redefines standing in for the ones redefined. A chain of orders carries
// a lifeline's events along it, so the readings are taken until none adds an event.
func (r *Renderer) inheritedOrders(orders []order, ends *lifelineEnds, lifelines []*Node) [][]timing {
	shown := make(map[*Node]*symbols.Symbol, len(ends.lifelines))
	for sym, lifeline := range ends.lifelines {
		shown[lifeline] = sym
	}
	held := make(map[lifelineEnd]bool, len(ends.proxies))
	for end := range ends.proxies {
		held[end] = true
	}
	// An order named through a lifeline puts its event there as a message does.
	for _, o := range orders {
		held[ends.orderEnd(o, o.from, o.fromThrough)], held[ends.orderEnd(o, o.to, o.toThrough)] = true, true
	}
	inherited := make([][]timing, len(orders))
	read := map[[2]int]bool{}
	for grew := true; grew; {
		grew = false
		for i, o := range orders {
			if o.fromThrough != nil || o.toThrough != nil || r.memberFor(o.within, o.from) == nil || r.memberFor(o.within, o.to) == nil {
				continue
			}
			for at, lifeline := range lifelines {
				if read[[2]int{i, at}] || !r.specializes(shown[lifeline], o.within) {
					continue
				}
				from := lifelineEnd{lifeline, r.memberFor(shown[lifeline], o.from)}
				to := lifelineEnd{lifeline, r.memberFor(shown[lifeline], o.to)}
				if from.occurrence == nil || to.occurrence == nil || !held[from] && !held[to] {
					continue
				}
				held[from], held[to] = true, true
				read[[2]int{i, at}] = true
				inherited[i] = append(inherited[i], timing{from: from, to: to, name: o.name})
				grew = true
			}
		}
	}
	for _, readings := range inherited {
		sort.SliceStable(readings, func(i, j int) bool {
			return slices.Index(lifelines, readings[i].from.lifeline) < slices.Index(lifelines, readings[j].from.lifeline)
		})
	}
	return inherited
}

// timing is an order between two events, each on the lifeline it is read on (none
// for an end no lifeline shows), and how a notice names it.
type timing struct {
	from, to lifelineEnd
	name     string
}

// succession is a timing drawn as an arrow: the timing it draws and its edge.
type succession struct {
	order int
	edge  Edge
}

// noticeSubject names an element a notice is about, by what it was declared as.
func (r *Renderer) noticeSubject(sym *symbols.Symbol) string {
	return r.subject(sym, declKind(sym))
}

// subject names an element a notice is about: what it is and its name, else what
// it is and where it is written, since anonymity is normal notation.
func (r *Renderer) subject(sym *symbols.Symbol, kind string) string {
	if sym.Named() && !sym.EffectiveName() {
		return kind + " " + localName(sym)
	}
	if owner := sym.Owner(); owner != nil {
		return fmt.Sprintf("%s %s in %s", article(kind), kind, r.notationName(owner))
	}
	return article(kind) + " " + kind
}

// article is the indefinite article a kind reads with.
func article(kind string) string {
	if kind != "" && strings.ContainsRune("aeiou", rune(kind[0])) {
		return "an"
	}
	return "a"
}

// messageKind is the keyword a message was written with — `message` or `flow`.
func messageKind(flow *symbols.Symbol) string {
	if usage, ok := flow.Decl.(*ast.Usage); ok && usage.Keyword != "" {
		return usage.Keyword
	}
	return declKind(flow)
}

// endText is an end of a message as it was written, for a notice — a feature
// chain segment by segment, so each segment is named as the notation names it.
func endText(end ast.Node) string {
	if text := qualifiedText(end); text != "" {
		return text
	}
	segments := strings.Split(lower.FeaturePath(end), ".")
	for i, segment := range segments {
		segments[i] = nameText(segment)
	}
	return strings.Join(segments, ".")
}

// messageLabel is a message's label as SysML v2.1 §8.2.3.9 message-label writes
// it: its declaration and `of` the payload it carries; else the type it is declared
// with, else the payload alone, else the keyword that declared it. An anonymous
// message is normal notation, so it is never left unlabeled.
func (r *Renderer) messageLabel(flow *symbols.Symbol) string {
	if flow.Named() && !flow.EffectiveName() {
		if payload := payloadText(flow); payload != "" {
			return localName(flow) + " of " + payload
		}
		return localName(flow)
	}
	if declared := declType(flow); declared != "" {
		return declared
	}
	if payload := payloadText(flow); payload != "" {
		return "of " + payload
	}
	return messageKind(flow)
}

// payloadText is what a flow carries as written, with the type a payload
// declared inline states — the `ignitionCmd : IgnitionCmd` of
// `message of ignitionCmd : IgnitionCmd from … to …`.
func payloadText(flow *symbols.Symbol) string {
	usage, _ := flow.Decl.(*ast.Usage)
	if usage == nil || usage.FlowEnds == nil {
		return ""
	}
	name := qualifiedText(usage.FlowEnds.Payload)
	if name == "" {
		return ""
	}
	text := notationName(name)
	if decl := usage.FlowEnds.PayloadDecl; decl != nil {
		if typ := typingOf(decl.Relationships); typ != "" {
			return text + " : " + typ
		}
	}
	return text
}

// declaredBefore orders two declarations the way a reader reads them: by the
// document they are in, then by their position in it.
func declaredBefore(a, b *symbols.Symbol) bool {
	if a.DocName != b.DocName {
		return a.DocName < b.DocName
	}
	return a.DeclSpan.Offset < b.DeclSpan.Offset
}

// orderMessages is the messages in the order the model states: each one a row
// its events share, sorted by the orders between those events (declaration order
// between rows nothing orders); a cycle is reported and declaration order stands.
func (r *Renderer) orderMessages(messages []message, successions []succession, orders []timing, out *Rendering) []Edge {
	occurrences := sequencedOccurrences(messages, orders)
	at := make(map[lifelineEnd]int, len(occurrences))
	for i, occurrence := range occurrences {
		at[occurrence] = i
	}
	stated := make([]timing, 0, len(orders))
	for _, o := range orders {
		if _, ok := at[o.from]; ok && o.from != o.to {
			if _, ok := at[o.to]; ok {
				stated = append(stated, o)
			}
		}
	}
	// The events first: an order against a message's own send → receive
	// direction cannot hold on one row however the rows are placed.
	eventsLater := make([][]int, len(occurrences))
	eventsIndegree := make([]int, len(occurrences))
	for _, msg := range messages {
		if i, j := at[msg.from], at[msg.to]; msg.fromTimed && msg.toTimed && i != j {
			eventsLater[i] = append(eventsLater[i], j)
			eventsIndegree[j]++
		}
	}
	for _, o := range stated {
		eventsLater[at[o.from]] = append(eventsLater[at[o.from]], at[o.to])
		eventsIndegree[at[o.to]]++
	}
	var cyclic []string
	ordered := func(a, b lifelineEnd) bool { return a == b }
	if placed, ok := stableTopological(eventsLater, eventsIndegree); !ok {
		cyclic = r.cyclicOrders(stated, at, placed)
	} else {
		cyclic = r.sortMessageRows(messages, occurrences, stated)
		ordered = reaches(eventsLater, at)
	}
	if cyclic != nil {
		out.Notices = append(out.Notices, fmt.Sprintf(
			"successions form a cycle (%s); the messages are shown in declaration order",
			strings.Join(cyclic, ", ")))
	}
	return sequenceRows(messages, successions, orders, ordered)
}

// reaches reports whether an event is another or is ordered before it, through
// the events' order graph: later lists each event's successors by index.
func reaches(later [][]int, at map[lifelineEnd]int) func(a, b lifelineEnd) bool {
	return func(a, b lifelineEnd) bool {
		if a == b {
			return true
		}
		from, ok := at[a]
		to, known := at[b]
		if !ok || !known {
			return false
		}
		seen := make([]bool, len(later))
		stack := []int{from}
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, j := range later[i] {
				if j == to {
					return true
				}
				if !seen[j] {
					seen[j] = true
					stack = append(stack, j)
				}
			}
		}
		return false
	}
}

// sequenceRows are the rows of the sequence in order: the messages as sorted, then
// each succession's arrow after the last row holding its source or an event ordered
// before it, else before the first row holding its target or an event ordered after
// it, else after the messages. A placed succession's row holds its two events, so a
// chain declared out of order still settles by its events; successions nothing orders
// keep declaration order. before is the events' order, identity when cyclic.
func sequenceRows(messages []message, successions []succession, orders []timing, before func(a, b lifelineEnd) bool) []Edge {
	type row struct {
		edge       Edge
		events     []lifelineEnd
		succession bool
	}
	rows := make([]row, 0, len(messages)+len(successions))
	for _, msg := range messages {
		rows = append(rows, row{edge: msg.edge, events: msg.events()})
	}
	for _, s := range successions {
		o := orders[s.order]
		after, ahead := -1, len(rows)
		for i, r := range rows {
			for _, held := range r.events {
				if before(held, o.from) {
					after = max(after, i)
				}
				if before(o.to, held) {
					ahead = min(ahead, i)
				}
			}
		}
		// After its source's row and the successions already there, in declaration order.
		place := ahead
		if after >= 0 {
			place = after + 1
			for place < ahead && rows[place].succession {
				place++
			}
		}
		rows = slices.Insert(rows, place, row{edge: s.edge, events: []lifelineEnd{o.from, o.to}, succession: true})
	}
	edges := make([]Edge, 0, len(rows))
	for _, r := range rows {
		edges = append(edges, r.edge)
	}
	return edges
}

// sortMessageRows sorts the messages by the orders between their rows, or
// reports the orders forming a cycle and leaves the messages as declared.
func (r *Renderer) sortMessageRows(messages []message, occurrences []lifelineEnd, stated []timing) []string {
	rowOf, row, rows := messageRows(messages, occurrences)
	later := make([][]int, rows)
	indegree := make([]int, rows)
	for _, o := range stated {
		if i, j := row[o.from], row[o.to]; i != j {
			later[i] = append(later[i], j)
			indegree[j]++
		}
	}
	sorted, ok := stableTopological(later, indegree)
	if !ok {
		return r.cyclicOrders(stated, row, sorted)
	}
	rank := make([]int, rows)
	for place, i := range sorted {
		rank[i] = place
	}
	place := make(map[*symbols.Symbol]int, len(messages))
	for i, msg := range messages {
		place[msg.flow] = rank[rowOf[i]]
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return place[messages[i].flow] < place[messages[j].flow]
	})
	return nil
}

// messageRows numbers the rows the orders sort: events on no message first (one
// each), then the messages in declaration order, those sharing an event as one.
func messageRows(messages []message, occurrences []lifelineEnd) ([]int, map[lifelineEnd]int, int) {
	parent := make([]int, len(messages))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	holder := make(map[lifelineEnd]int, 2*len(messages))
	for i, msg := range messages {
		for _, end := range msg.events() {
			if j, ok := holder[end]; ok {
				parent[find(i)] = find(j)
			} else {
				holder[end] = i
			}
		}
	}
	rows := 0
	row := make(map[lifelineEnd]int, len(occurrences))
	for _, occurrence := range occurrences {
		if _, held := holder[occurrence]; !held {
			row[occurrence] = rows
			rows++
		}
	}
	numbered := make([]int, len(messages))
	rowOf := make([]int, len(messages))
	for i := range messages {
		root := find(i)
		if numbered[root] == 0 {
			rows++
			numbered[root] = rows
		}
		rowOf[i] = numbered[root] - 1
	}
	for end, i := range holder {
		row[end] = rowOf[i]
	}
	return rowOf, row, rows
}

// events are the ends of a message that are events of their own on a lifeline.
func (m message) events() []lifelineEnd {
	var events []lifelineEnd
	if m.fromTimed {
		events = append(events, m.from)
	}
	if m.toTimed {
		events = append(events, m.to)
	}
	return events
}

// sequencedOccurrences are the occurrences the message order is sorted over: the
// ones the messages run between, and the ones an order joins to those, in
// declaration order so that nothing stated leaves the order to chance.
func sequencedOccurrences(messages []message, orders []timing) []lifelineEnd {
	sequenced := map[lifelineEnd]bool{}
	for _, msg := range messages {
		sequenced[msg.from], sequenced[msg.to] = true, true
	}
	for grew := true; grew; {
		grew = false
		for _, o := range orders {
			if o.from.occurrence == nil || o.to.occurrence == nil || sequenced[o.from] == sequenced[o.to] {
				continue
			}
			sequenced[o.from], sequenced[o.to] = true, true
			grew = true
		}
	}
	out := make([]lifelineEnd, 0, len(sequenced))
	for occurrence := range sequenced {
		if occurrence.occurrence != nil {
			out = append(out, occurrence)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].before(out[j]) })
	return out
}

// cyclicOrders names the orders the sort could not satisfy: those with an end it
// could not place, which is where the cycle is.
func (r *Renderer) cyclicOrders(orders []timing, at map[lifelineEnd]int, placed []int) []string {
	sorted := make(map[int]bool, len(placed))
	for _, i := range placed {
		sorted[i] = true
	}
	var names []string
	for _, o := range orders {
		if sorted[at[o.from]] && sorted[at[o.to]] {
			continue
		}
		name := o.name
		if name == "" {
			name = fmt.Sprintf("%s to %s",
				localName(o.from.occurrence), localName(o.to.occurrence))
		}
		names = append(names, name)
	}
	return names
}

// stableTopological orders the occurrences so that every stated order runs
// forwards, always taking the earliest-declared one that nothing is waiting on,
// so declaration order stands wherever nothing states otherwise. A graph holding
// a cycle is false, with the part that could be sorted returned.
func stableTopological(later [][]int, indegree []int) ([]int, bool) {
	sorted := make([]int, 0, len(later))
	taken := make([]bool, len(later))
	for len(sorted) < len(later) {
		next := -1
		for i := range later {
			if !taken[i] && indegree[i] == 0 {
				next = i
				break
			}
		}
		if next < 0 {
			return sorted, false
		}
		taken[next] = true
		sorted = append(sorted, next)
		for _, after := range later[next] {
			indegree[after]--
		}
	}
	return sorted, true
}

// isSuccessionUsage reports whether a symbol is a succession usage stating the
// two occurrences it orders, which is what a `succession first a then b;` is.
func isSuccessionUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageSuccession
}

// occurrenceContainer reports whether an element can declare the occurrences of
// an interaction in its body, in which case those are the lifelines and the
// container itself is not one.
func occurrenceContainer(sym *symbols.Symbol) bool {
	if semantics.IsSubjectUsage(sym) {
		return true
	}
	switch sym.Kind {
	case symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage,
		symbols.SymbolPartDef, symbols.SymbolPartUsage:
		return true
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		return usage.Kind == ast.UsageInteraction
	}
	return false
}

// behaviorLike reports whether an element is behavior an interaction performs
// rather than a participant in it, whose flows are its own action flow.
func behaviorLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolActionDef, symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	}
	return sym.Kind.IsActionUsage()
}

// lifelineLike reports whether an element can hold an occurrence, which is what
// a sequence rendering draws a lifeline for. An attribute, a port, a package or
// a view holds none, and is reported rather than drawn.
func lifelineLike(sym *symbols.Symbol) bool {
	if semantics.IsSubjectUsage(sym) {
		// A case's subject holds events as the part it refers to does.
		return true
	}
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage,
		symbols.SymbolActionDef, symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	}
	return sym.Kind.IsActionUsage()
}
