package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// renderSequence renders the occurrences of an interaction as lifelines and the
// flows between them as directed messages, ordered as the model states. A
// lifeline list is flat — a sequence diagram nests nothing — so a message
// attaching to an event nested in a lifeline is drawn on that lifeline. Anything
// a sequence rendering cannot show is reported rather than dropped.
func (r *Renderer) renderSequence(exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	lifelines := map[*symbols.Symbol]*Node{}
	stated := &interactions{}
	visited := map[*symbols.Symbol]bool{}
	for _, elem := range exposed {
		switch {
		case isFlowUsage(elem):
			stated.addFlow(elem)
		case isSuccessionUsage(elem):
			r.addSuccessionUsage(elem, stated)
		case r.model.IsConnectorUsage(elem):
			stated.addConnector(elem)
		case occurrenceContainer(elem), lifelineLike(elem):
			for _, participant := range lifelinesOf(elem) {
				if lifelines[participant] != nil {
					continue
				}
				name := r.notationName(participant)
				if participant != elem {
					name = localName(participant)
				}
				node := &Node{ID: ids.take(), Kind: declKind(participant), Name: name,
					Type: declType(participant), Origin: symbolOrigin(participant)}
				out.Roots = append(out.Roots, node)
				lifelines[participant] = node
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
	out.Edges = r.orderMessages(r.messages(stated.flows, lifelines, out), stated.orders, out)
}

// lifelinesOf are the lifelines an exposed element contributes: the occurrences
// an interaction declares in its body, else the element itself when it declares
// none. The container is not a lifeline of its own interaction, and neither is
// behavior it performs, which a sequence rendering shows as messages instead.
func lifelinesOf(elem *symbols.Symbol) []*symbols.Symbol {
	if occurrenceContainer(elem) {
		var participants []*symbols.Symbol
		for _, member := range containedMembers(elem) {
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
	from, to *symbols.Symbol
	edge     Edge
}

// order is a sequence the model states between two occurrences, and how a notice
// names it.
type order struct {
	from, to *symbols.Symbol
	name     string
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
	for _, member := range containedMembers(sym) {
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
	for _, member := range containedMembers(sym) {
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
	if succession.Name != "" && !succession.EffectiveName() {
		name = localName(succession)
	}
	into.orders = append(into.orders, order{from: from, to: to, name: name})
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
		into.orders = append(into.orders, order{from: from, to: to})
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
	for _, candidate := range containedMembers(owner) {
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
func (r *Renderer) messages(flows []*symbols.Symbol, lifelines map[*symbols.Symbol]*Node, out *Rendering) []message {
	var messages []message
	for _, flow := range flows {
		usage, _ := flow.Decl.(*ast.Usage)
		if usage == nil || usage.FlowEnds == nil || usage.FlowEnds.From == nil || usage.FlowEnds.To == nil {
			out.Notices = append(out.Notices, fmt.Sprintf("%s states no source and target; no message is drawn",
				r.subject(flow, messageKind(flow))))
			continue
		}
		var ends []*symbols.Symbol
		var nodes []*Node
		for _, end := range []ast.Node{usage.FlowEnds.From, usage.FlowEnds.To} {
			occurrence, node := r.messageEnd(flow, end, lifelines)
			if node == nil {
				out.Notices = append(out.Notices, fmt.Sprintf(
					"%s attaches to %s, which is on no lifeline the rendering shows; no message is drawn",
					r.subject(flow, messageKind(flow)), endText(end)))
				break
			}
			ends, nodes = append(ends, occurrence), append(nodes, node)
		}
		if len(nodes) != 2 {
			continue
		}
		messages = append(messages, message{
			flow: flow, from: ends[0], to: ends[1],
			edge: Edge{From: nodes[0].ID, To: nodes[1].ID, Label: r.messageLabel(flow), Kind: EdgeFlow,
				Origin: symbolOrigin(flow)},
		})
	}
	sort.SliceStable(messages, func(i, j int) bool { return declaredBefore(messages[i].flow, messages[j].flow) })
	return messages
}

// messageEnd is the occurrence an end of a message names and the lifeline the
// arrow is drawn on: the whole feature chain first, then a segment shorter, then
// the nearest owner shown — `producer.publish_source_event` is an event of the
// `producer` lifeline.
func (r *Renderer) messageEnd(flow *symbols.Symbol, end ast.Node, lifelines map[*symbols.Symbol]*Node) (*symbols.Symbol, *Node) {
	for attachment := end; attachment != nil; attachment = chainOperand(attachment) {
		target, ok := r.resolver.ResolveTarget(flow.OwnerScope, attachment)
		if !ok {
			continue
		}
		for sym := target; sym != nil; sym = sym.Owner() {
			if node, ok := lifelines[sym]; ok {
				return target, node
			}
		}
	}
	return nil, nil
}

// noticeSubject names an element a notice is about, by what it was declared as.
func (r *Renderer) noticeSubject(sym *symbols.Symbol) string {
	return r.subject(sym, declKind(sym))
}

// subject names an element a notice is about: what it is and its name, else what
// it is and where it is written, since anonymity is normal notation.
func (r *Renderer) subject(sym *symbols.Symbol, kind string) string {
	if sym.Name != "" && !sym.EffectiveName() {
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

// messageLabel names a message: its own name, else the type it is declared with,
// else the payload it carries as written, else the keyword that declared it. An
// anonymous message is normal notation, so it is never left unlabeled.
func (r *Renderer) messageLabel(flow *symbols.Symbol) string {
	if flow.Name != "" && !flow.EffectiveName() {
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

// orderMessages is the messages in the order the model states. The model states
// no order over messages directly: it orders the occurrences they run between,
// so the occurrences are sorted and each message takes the place of the one it
// runs from. A cycle among the stated orders cannot be sorted, so it is reported
// and declaration order stands for the whole rendering.
func (r *Renderer) orderMessages(messages []message, orders []order, out *Rendering) []Edge {
	occurrences := sequencedOccurrences(messages, orders)
	at := make(map[*symbols.Symbol]int, len(occurrences))
	for i, occurrence := range occurrences {
		at[occurrence] = i
	}
	later := make([][]int, len(occurrences))
	indegree := make([]int, len(occurrences))
	constrain := func(from, to *symbols.Symbol) bool {
		i, iOK := at[from]
		j, jOK := at[to]
		if !iOK || !jOK || i == j {
			return false
		}
		later[i] = append(later[i], j)
		indegree[j]++
		return true
	}
	for _, msg := range messages {
		constrain(msg.from, msg.to)
	}
	stated := make([]order, 0, len(orders))
	for _, o := range orders {
		if constrain(o.from, o.to) {
			stated = append(stated, o)
		}
	}
	if sorted, ok := stableTopological(later, indegree); ok {
		rank := make(map[*symbols.Symbol]int, len(sorted))
		for place, i := range sorted {
			rank[occurrences[i]] = place
		}
		// A row states order at both ends, so the receiving occurrence ranks first;
		// each message's own from → to edge carries the sending order into it.
		sort.SliceStable(messages, func(i, j int) bool {
			if rank[messages[i].to] != rank[messages[j].to] {
				return rank[messages[i].to] < rank[messages[j].to]
			}
			return rank[messages[i].from] < rank[messages[j].from]
		})
	} else {
		out.Notices = append(out.Notices, fmt.Sprintf(
			"successions form a cycle (%s); the messages are shown in declaration order",
			strings.Join(r.cyclicOrders(stated, at, sorted), ", ")))
	}
	edges := make([]Edge, 0, len(messages))
	for _, msg := range messages {
		edges = append(edges, msg.edge)
	}
	return edges
}

// sequencedOccurrences are the occurrences the message order is sorted over: the
// ones the messages run between, and the ones an order joins to those, in
// declaration order so that nothing stated leaves the order to chance.
func sequencedOccurrences(messages []message, orders []order) []*symbols.Symbol {
	sequenced := map[*symbols.Symbol]bool{}
	for _, msg := range messages {
		sequenced[msg.from], sequenced[msg.to] = true, true
	}
	for grew := true; grew; {
		grew = false
		for _, o := range orders {
			if o.from == nil || o.to == nil || sequenced[o.from] == sequenced[o.to] {
				continue
			}
			sequenced[o.from], sequenced[o.to] = true, true
			grew = true
		}
	}
	out := make([]*symbols.Symbol, 0, len(sequenced))
	for occurrence := range sequenced {
		if occurrence != nil {
			out = append(out, occurrence)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return declaredBefore(out[i], out[j]) })
	return out
}

// cyclicOrders names the orders the sort could not satisfy: those with an end it
// could not place, which is where the cycle is.
func (r *Renderer) cyclicOrders(orders []order, at map[*symbols.Symbol]int, placed []int) []string {
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
				localName(o.from), localName(o.to))
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
	case symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	}
	return false
}

// lifelineLike reports whether an element can hold an occurrence, which is what
// a sequence rendering draws a lifeline for. An attribute, a port, a package or
// a view holds none, and is reported rather than drawn.
func lifelineLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage,
		symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	}
	return false
}
