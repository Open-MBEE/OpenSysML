package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Static partial-order reduction: from each state the search explores a
// persistent set of the enabled moves, closed under "a unit whose future may
// not commute with a member is a member", less a sleep set of the moves an
// equivalent predecessor already explored (bounded-model-checking.md, "The algorithm").
// A unit is what a move advances: an action's token, or a state machine as a whole.

// futureKey identifies what a token may still do: its node in its flow.
type futureKey struct {
	graph *lower.ActionGraph
	node  ast.Node
}

// unit is the unit the move advances: its owner's token, the owner itself for a
// state machine's move.
func (m enabledMove) unit() tokenKey {
	return tokenKey{owner: m.Owner, id: m.Token}
}

// footprintOf is the footprint of the move: what advancing the token over its
// node touches, what a dispatch may fire, or what a do step runs. A move the
// executor will refuse depends on everything.
func (c *checker) footprintOf(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	switch exec := m.Owner.(type) {
	case *ActionExecutor:
		return c.standing(exec, exec.tokens[exec.tokenIndex(m.Token)])
	case *StateExecutor:
		if m.Kind == moveDoStep {
			return doStepFootprint(exec, m.Node)
		}
		return dispatchFootprint(exec)
	}
	return lower.Footprint{Dynamic: true}
}

// standing is the footprint of the node the token stands at; a body paused
// mid-statement goes on with the rest of that node's, which the node's covers.
func (c *checker) standing(exec *ActionExecutor, t Token) lower.Footprint {
	if exec.dynamics != nil {
		return dynamicsFootprint()
	}
	return tokenGraphOf(exec, t).Footprints()[t.Location]
}

// turnFootprint is what the move's owner may touch over the turn the move takes:
// the owner keeps the turn until it has no move left at the instant, so it is the
// future of every unit of the owner — each token of an action, the machine whole.
func (c *checker) turnFootprint(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	switch exec := m.Owner.(type) {
	case *ActionExecutor:
		var turn lower.Footprint
		for _, t := range exec.tokens {
			turn = unionFootprints(turn, c.tokenFuture(exec, t))
		}
		return turn
	case *StateExecutor:
		return c.machineFuture(exec)
	}
	return lower.Footprint{Dynamic: true}
}

// machineStanding is what the machine's next unit may touch: a dispatch out of
// its configuration, or a step of a do behavior of it.
func machineStanding(e *StateExecutor) lower.Footprint {
	fp := dispatchFootprint(e)
	for _, act := range e.doActions {
		fp = unionFootprints(fp, doStepFootprint(e, act.state))
	}
	return fp
}

// dispatchFootprint is what dispatching the machine's next event may touch:
// firing any transition out of the states of its active configuration and those
// enclosing them, and taking a message off the bus where one of those states
// accepts, calls or defers one — a machine at rest reads nothing.
func dispatchFootprint(e *StateExecutor) lower.Footprint {
	if e.activeConfig == nil {
		return lower.Footprint{}
	}
	var fp lower.Footprint
	footprints := e.graph.TransitionFootprints()
	seen := make(map[*ast.StateNode]bool)
	for _, leaf := range e.activeStates() {
		for _, state := range e.getParentChain(leaf) {
			if seen[state] {
				continue
			}
			seen[state] = true
			for _, trans := range e.graph.Transitions[state] {
				fp = unionFootprints(fp, footprints[trans])
				if channel, takes := triggerChannel(trans); takes {
					fp.Accepts = append(fp.Accepts, channel)
				}
			}
			if len(e.graph.Deferred[state]) > 0 {
				fp.Accepts = append(fp.Accepts, lower.Channel{})
			}
		}
	}
	return fp
}

// triggerChannel is the bus channel a transition's trigger takes its occurrence
// from, for an accept or call trigger; none for a completion, time or change one.
func triggerChannel(trans *lower.Transition) (lower.Channel, bool) {
	switch t := trans.Trigger.(type) {
	case *ast.AcceptEvent:
		channel := lower.Channel{Port: trans.Via}
		if typed := ast.AsQualifiedName(t.SignalType); typed != nil {
			channel.Signal = lower.FeaturePath(typed)
		}
		return channel, true
	case *ast.CallEvent:
		return lower.Channel{Port: trans.Via}, true
	}
	return lower.Channel{}, false
}

// doStepFootprint is what one step of the state's do behavior runs: the rest of
// the behavior paused mid-way, covered by that behavior's, or the next one pending.
func doStepFootprint(e *StateExecutor, state ast.Node) lower.Footprint {
	for _, act := range e.doActions {
		if act.state != state {
			continue
		}
		if act.run != nil {
			return e.graph.BehaviorFootprints()[act.run.host.behavior.Node]
		}
		if len(act.pending) == 0 {
			return lower.Footprint{}
		}
		return e.graph.BehaviorFootprints()[act.pending[0].Node]
	}
	return lower.Footprint{}
}

// machineFuture is the footprint of every move the machine may make from any
// configuration: its transitions' and its behaviors', with the messages any
// trigger of it takes; memoized by graph.
func (c *checker) machineFuture(e *StateExecutor) lower.Footprint {
	if fp, ok := c.machineFutures[e.graph]; ok {
		return fp
	}
	var future lower.Footprint
	for trans, fp := range e.graph.TransitionFootprints() {
		future = unionFootprints(future, fp)
		if channel, takes := triggerChannel(trans); takes {
			future.Accepts = append(future.Accepts, channel)
		}
	}
	for _, fp := range e.graph.BehaviorFootprints() {
		future = unionFootprints(future, fp)
	}
	if len(e.graph.Deferred) > 0 {
		future.Accepts = append(future.Accepts, lower.Channel{})
	}
	c.machineFutures[e.graph] = future
	return future
}

// tokenFuture is what the token may still touch: every node reachable from its
// own, the one it stands at included, and from the nodes its frames stand at.
func (c *checker) tokenFuture(exec *ActionExecutor, t Token) lower.Footprint {
	if exec.dynamics != nil {
		return dynamicsFootprint()
	}
	future := c.reach(tokenGraphOf(exec, t), t.Location)
	for frame := t.frame; frame != nil && frame.node != nil; frame = frame.parent {
		flow := frame.flow
		if flow == nil && frame.parent != nil {
			flow = frame.parent.graph
		}
		if flow == nil {
			flow = exec.graph
		}
		future = unionFootprints(future, c.reach(flow, frame.node))
	}
	return future
}

func tokenGraphOf(exec *ActionExecutor, t Token) *lower.ActionGraph {
	if t.frame != nil && t.frame.graph != nil {
		return t.frame.graph
	}
	return exec.graph
}

// reach is the union of the footprints of every node reachable from node in
// graph over its successions, the flows those nodes own included; memoized.
func (c *checker) reach(graph *lower.ActionGraph, node ast.Node) lower.Footprint {
	key := futureKey{graph: graph, node: node}
	if fp, ok := c.futures[key]; ok {
		return fp
	}
	var future lower.Footprint
	seen := make(map[ast.Node]bool)
	work := []ast.Node{node}
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		future = unionFootprints(future, graph.Footprints()[n])
		if sub, owns := graph.Subflows[n]; owns && sub != nil && sub.Graph != nil && sub.Graph.Initial != nil {
			future = unionFootprints(future, c.reach(sub.Graph, sub.Graph.Initial))
		}
		for _, edge := range graph.Edges[n] {
			work = append(work, edge.Target)
		}
	}
	c.futures[key] = future
	return future
}

func unionFootprints(f, g lower.Footprint) lower.Footprint {
	return lower.Footprint{
		Reads:      append(slices.Clone(f.Reads), g.Reads...),
		Writes:     append(slices.Clone(f.Writes), g.Writes...),
		Sends:      append(slices.Clone(f.Sends), g.Sends...),
		Accepts:    append(slices.Clone(f.Accepts), g.Accepts...),
		Control:    append(slices.Clone(f.Control), g.Control...),
		Completion: f.Completion || g.Completion,
		Dynamic:    f.Dynamic || g.Dynamic,
	}
}

// dependent reports whether the two moves may not commute: the moves of one
// unit never do, and otherwise their footprints decide.
func dependent(a, b searchMove) bool {
	return a.unit() == b.unit() || a.footprint.Dependent(b.footprint)
}

// persistent selects the moves to explore from a state, less the moves asleep;
// every move when unreduced or under a property, which reads what no footprint
// names. The set is grown from the first awake move: a
// unit whose future may not commute with a member's move joins with every move
// it has; one with none (parked, or waiting at a join) brings in the units
// whose future may let it go on, since a schedule outside the set could
// otherwise reach its dependent moves.
func (c *checker) persistent(all, sleep []searchMove) []searchMove {
	if !c.opts.Reduce || len(c.props) > 0 {
		return slices.Clone(all)
	}
	awake := func(m searchMove) bool {
		return !slices.ContainsFunc(sleep, m.same)
	}
	first := slices.IndexFunc(all, awake)
	if first < 0 {
		return nil
	}
	closure := newPersistentClosure(c, all)
	closure.include(all[first].unit())
	for grew := true; grew; {
		grew = false
		for _, u := range closure.units {
			if closure.in[u] {
				continue
			}
			if closure.futureDependsOnSet(u) {
				closure.include(u)
				grew = true
			}
		}
	}
	// In canonical order, awake members only.
	out := make([]searchMove, 0, len(all))
	for _, m := range all {
		if closure.in[m.unit()] && awake(m) {
			out = append(out, m)
		}
	}
	return out
}

// persistentClosure is a persistent set under construction: the units in it,
// the moves of the state and the footprints of every unit of the state.
type persistentClosure struct {
	c        *checker
	all      []searchMove
	units    []tokenKey
	in       map[tokenKey]bool
	standing map[tokenKey]lower.Footprint
	future   map[tokenKey]lower.Footprint
}

func newPersistentClosure(c *checker, all []searchMove) *persistentClosure {
	p := &persistentClosure{
		c:        c,
		all:      all,
		in:       make(map[tokenKey]bool),
		standing: make(map[tokenKey]lower.Footprint),
		future:   make(map[tokenKey]lower.Footprint),
	}
	for _, exec := range c.inv.executors() {
		switch e := exec.(type) {
		case *ActionExecutor:
			for _, t := range e.tokens {
				u := tokenKey{owner: exec, id: t.ID}
				p.units = append(p.units, u)
				p.standing[u] = c.standing(e, t)
				p.future[u] = c.tokenFuture(e, t)
			}
		case *StateExecutor:
			u := tokenKey{owner: exec}
			p.units = append(p.units, u)
			p.standing[u] = machineStanding(e)
			p.future[u] = c.machineFuture(e)
		default:
			u := tokenKey{owner: exec}
			p.units = append(p.units, u)
			p.standing[u] = lower.Footprint{Dynamic: true}
			p.future[u] = lower.Footprint{Dynamic: true}
		}
	}
	for _, m := range all {
		if m.Fails != nil {
			p.standing[m.unit()] = lower.Footprint{Dynamic: true}
			p.future[m.unit()] = lower.Footprint{Dynamic: true}
		}
	}
	return p
}

// include adds a unit: its moves when it has some, else the units whose
// future may let it go on.
func (p *persistentClosure) include(u tokenKey) {
	if p.in[u] {
		return
	}
	p.in[u] = true
	if slices.ContainsFunc(p.all, func(m searchMove) bool { return m.unit() == u }) {
		return
	}
	standing := p.standing[u]
	for _, other := range p.units {
		if !p.in[other] && p.future[other].Dependent(standing) {
			p.include(other)
		}
	}
}

// futureDependsOnSet reports whether the unit's future may not commute with
// a move of the set.
func (p *persistentClosure) futureDependsOnSet(u tokenKey) bool {
	future := p.future[u]
	for _, m := range p.all {
		if p.in[m.unit()] && future.Dependent(m.footprint) {
			return true
		}
	}
	return false
}

// childSleep is the sleep set of the state the move reaches: the moves asleep
// or already explored in the parent that commute with it.
func (c *checker) childSleep(f *checkFrame, m searchMove) []searchMove {
	if !c.opts.Reduce {
		return nil
	}
	var sleep []searchMove
	for _, s := range f.sleep {
		if !dependent(s, m) {
			sleep = append(sleep, s)
		}
	}
	for _, s := range f.moves[:f.next-1] {
		if !dependent(s, m) {
			sleep = append(sleep, s)
		}
	}
	return sleep
}
