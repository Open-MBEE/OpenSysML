package lower

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// SequenceGraph is an occurrence definition or usage lowered as an interaction:
// the lifelines (the usages of its body holding event occurrences), the events
// each holds, the orders the model states between events, and its messages as
// transfers in the order the events admit. A message is abstract (SysML v2.1
// §7.16.2) and is realized by a transfer between its two ends (§7.16.1).
type SequenceGraph struct {
	Decl      ast.Node
	Scope     *symbols.Scope
	Lifelines []*symbols.Symbol
	Events    []*SequenceEvent
	Orders    []SequenceOrder
	// Order is the events in realization order: topological over the stated
	// orders and each message's own send-before-receive, ties broken by
	// declaration order. A transfer is posted at its source event and taken at
	// its target event.
	Order []*SequenceEvent
	// Transfers are the messages in the order they are taken, which is the order
	// of their target events.
	Transfers []*Transfer
}

// SequenceEvent is one event occurrence a lifeline holds, with the action its
// body performs when the event occurs (`perform action spinCall ::> Motor::spin`), if any.
type SequenceEvent struct {
	Symbol   *symbols.Symbol
	Lifeline *symbols.Symbol
	Perform  *symbols.Symbol
	// Sends and Receives are the transfers leaving and reaching the event, in declaration order.
	Sends, Receives []*Transfer
	position        int
}

// SequenceOrder is one order stated between two events: a member-attached `then`
// or a `succession first a then b;`.
type SequenceOrder struct {
	From, To *SequenceEvent
	Decl     ast.Node
}

// Transfer is one message realized as a transfer from the event it leaves to the
// event it reaches, carrying the payload its `of` clause states.
type Transfer struct {
	Message  *symbols.Symbol
	Name     string
	From, To *SequenceEvent
	Payload  *ast.FlowEnds
	position int
}

var (
	// ErrNoSequence is an occurrence whose body holds no lifeline with an event occurrence.
	ErrNoSequence = errors.New("no lifeline of the occurrence holds an event occurrence")
	// ErrMessageEndUnowned is a message end naming no event occurrence of a lifeline.
	ErrMessageEndUnowned = errors.New("message end names no event occurrence of a lifeline")
	// ErrSequenceCycle is a set of orders no realization order satisfies.
	ErrSequenceCycle = errors.New("the orders between the events form a cycle")
)

// ToSequenceGraph lowers an occurrence definition or usage read as an interaction.
// Resolution of the ends needs the resolver; nil lowers a body whose ends are all
// simple names of the occurrence's own members.
func ToSequenceGraph(decl ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (*SequenceGraph, error) {
	if scope == nil {
		return nil, fmt.Errorf("%w: %s declares no body", ErrNoSequence, occurrenceText(decl))
	}
	g := &SequenceGraph{Decl: decl, Scope: scope}
	l := &sequenceLowerer{graph: g, resolver: resolver, events: map[*symbols.Symbol]*SequenceEvent{}}
	for _, member := range scope.Members() {
		if sequenceLifeline(member) {
			l.lifeline(member)
		}
	}
	if len(g.Events) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoSequence, occurrenceText(decl))
	}
	// A succession or a message may be anonymous, which Members leaves out.
	for _, member := range scope.AllMembers() {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageSuccession {
			l.successionUsage(member, usage)
		}
	}
	for _, line := range g.Lifelines {
		l.successionEdges(line)
	}
	for _, member := range scope.AllMembers() {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageFlow || usage.FlowEnds == nil {
			continue
		}
		if err := l.message(member, usage); err != nil {
			return nil, err
		}
	}
	if err := l.order(); err != nil {
		return nil, err
	}
	return g, nil
}

type sequenceLowerer struct {
	graph    *SequenceGraph
	resolver *resolve.Resolver
	events   map[*symbols.Symbol]*SequenceEvent
}

// sequenceLifeline reports whether a member of the occurrence can hold the events
// of a lifeline: a part, item, occurrence or individual usage.
func sequenceLifeline(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartUsage, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage:
		return sym.Scope != nil
	}
	return false
}

// lifeline records the lifeline's event occurrences in declaration order; a
// usage holding none is structure, not a lifeline.
func (l *sequenceLowerer) lifeline(line *symbols.Symbol) {
	var events []*SequenceEvent
	for _, member := range line.Scope.Members() {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !usage.IsEvent {
			continue
		}
		ev := &SequenceEvent{Symbol: member, Lifeline: line, position: len(l.graph.Events) + len(events)}
		if member.Scope != nil {
			for _, nested := range member.Scope.Members() {
				if nested.Kind == symbols.SymbolActionUsage {
					ev.Perform = nested
					break
				}
			}
		}
		events = append(events, ev)
	}
	if len(events) == 0 {
		return
	}
	l.graph.Lifelines = append(l.graph.Lifelines, line)
	for _, ev := range events {
		l.events[ev.Symbol] = ev
		l.graph.Events = append(l.graph.Events, ev)
	}
}

// successionUsage records the order a `succession first a then b;` of the
// occurrence states, where both ends are events; one between anything else
// (an occurrence usage of a nested interaction) orders no transfer.
func (l *sequenceLowerer) successionUsage(sym *symbols.Symbol, usage *ast.Usage) {
	if len(usage.ConnectorEnds) != 2 {
		return
	}
	from := l.event(sym.OwnerScope, usage.ConnectorEnds[0].AttachedTarget())
	to := l.event(sym.OwnerScope, usage.ConnectorEnds[1].AttachedTarget())
	if from != nil && to != nil {
		l.graph.Orders = append(l.graph.Orders, SequenceOrder{From: from, To: to, Decl: usage})
	}
}

// successionEdges records the orders a lifeline's member-attached `then` states
// between its events (`event occurrence a; then event occurrence b;`).
func (l *sequenceLowerer) successionEdges(line *symbols.Symbol) {
	for _, member := range bodyMembers(line.Decl) {
		edge, ok := member.(*ast.SuccessionEdge)
		if !ok {
			continue
		}
		from := l.edgeEnd(line, edge.Source, edge.SourceMember)
		to := l.edgeEnd(line, edge.Target, edge.TargetMember)
		if from != nil && to != nil {
			l.graph.Orders = append(l.graph.Orders, SequenceOrder{From: from, To: to, Decl: edge})
		}
	}
}

// edgeEnd is the event an end of a succession edge names, or the member the
// notation attached it to when it names none.
func (l *sequenceLowerer) edgeEnd(line *symbols.Symbol, name *ast.QualifiedName, member ast.Node) *SequenceEvent {
	if name != nil && len(name.Parts) > 0 {
		return l.event(line.Scope, name)
	}
	member = unwrapMembership(member)
	for _, candidate := range line.Scope.Members() {
		if member != nil && candidate.Decl == member {
			return l.events[candidate]
		}
	}
	return nil
}

// event is the event an end names, resolved where the end is written: the whole
// chain (`producer.publish_source_event`), else nothing.
func (l *sequenceLowerer) event(scope *symbols.Scope, end ast.Node) *SequenceEvent {
	if end == nil {
		return nil
	}
	if l.resolver != nil {
		if target, ok := l.resolver.ResolveTarget(scope, end); ok {
			return l.events[target]
		}
		return nil
	}
	segments := endSegments(end)
	if len(segments) == 0 {
		return nil
	}
	for sym, ev := range l.events {
		if sym.Name == segments[len(segments)-1] && (len(segments) == 1 || ev.Lifeline.Name == segments[len(segments)-2]) {
			return ev
		}
	}
	return nil
}

// message records a flow usage of the occurrence as a transfer between the events
// its ends name; an end naming no event of a lifeline is a typed error.
func (l *sequenceLowerer) message(sym *symbols.Symbol, usage *ast.Usage) error {
	name, _ := ast.EffectiveName(usage)
	from := l.event(sym.OwnerScope, ast.EndTarget(usage.FlowEnds.From))
	to := l.event(sym.OwnerScope, ast.EndTarget(usage.FlowEnds.To))
	for i, end := range []*SequenceEvent{from, to} {
		if end == nil {
			which, text := "source", usage.FlowEnds.From
			if i == 1 {
				which, text = "target", usage.FlowEnds.To
			}
			return fmt.Errorf("%w: the %s %s of message %s", ErrMessageEndUnowned, which, flowEndText(text), orAnonymous(name))
		}
	}
	t := &Transfer{Message: sym, Name: orAnonymous(name), From: from, To: to, Payload: usage.FlowEnds, position: len(l.graph.Transfers)}
	l.graph.Transfers = append(l.graph.Transfers, t)
	from.Sends = append(from.Sends, t)
	to.Receives = append(to.Receives, t)
	return nil
}

// order places the events in a stable topological order over the stated orders
// and each message's send-before-receive, and sorts the transfers by the
// position of the event each reaches, declaration order breaking ties.
func (l *sequenceLowerer) order() error {
	g := l.graph
	n := len(g.Events)
	later := make([][]int, n)
	indegree := make([]int, n)
	add := func(from, to *SequenceEvent) {
		later[from.position] = append(later[from.position], to.position)
		indegree[to.position]++
	}
	for _, o := range g.Orders {
		add(o.From, o.To)
	}
	for _, t := range g.Transfers {
		if t.From != t.To {
			add(t.From, t.To)
		}
	}
	at := make([]int, n)
	placed := 0
	ready := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if indegree[i] == 0 {
			ready = append(ready, i)
		}
	}
	for len(ready) > 0 {
		sort.Ints(ready)
		i := ready[0]
		ready = ready[1:]
		at[i] = placed
		placed++
		g.Order = append(g.Order, g.Events[i])
		for _, j := range later[i] {
			indegree[j]--
			if indegree[j] == 0 {
				ready = append(ready, j)
			}
		}
	}
	if placed < n {
		var stuck []string
		for i := 0; i < n; i++ {
			if indegree[i] > 0 {
				stuck = append(stuck, g.Events[i].Lifeline.Name+"."+g.Events[i].Symbol.Name)
			}
		}
		return fmt.Errorf("%w: %s", ErrSequenceCycle, strings.Join(stuck, ", "))
	}
	sort.SliceStable(g.Transfers, func(i, j int) bool {
		a, b := g.Transfers[i], g.Transfers[j]
		if at[a.To.position] != at[b.To.position] {
			return at[a.To.position] < at[b.To.position]
		}
		return a.position < b.position
	})
	return nil
}

// bodyMembers are the members a declaration's body writes, edges included, each
// unwrapped from its membership.
func bodyMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	}
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		out = append(out, unwrapMembership(member))
	}
	return out
}

// occurrenceText names an occurrence for a diagnostic.
func occurrenceText(decl ast.Node) string {
	switch d := decl.(type) {
	case *ast.Definition:
		return "occurrence def " + orAnonymous(d.Ident.Name)
	case *ast.Usage:
		name, _ := ast.EffectiveName(d)
		return "occurrence " + orAnonymous(name)
	}
	return "occurrence"
}
