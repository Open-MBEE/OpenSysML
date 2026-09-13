package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
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
// node touches. A move the executor will refuse depends on everything, as does a
// state machine's move until its footprints are projected.
func (c *checker) footprintOf(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	exec, isAction := m.Owner.(*ActionExecutor)
	if !isAction {
		return lower.Footprint{Dynamic: true}
	}
	return c.standing(exec, exec.tokens[exec.tokenIndex(m.Token)])
}

// standing is the footprint of the node the token stands at.
func (c *checker) standing(exec *ActionExecutor, t Token) lower.Footprint {
	if t.body != nil {
		return lower.Footprint{Dynamic: true}
	}
	return tokenGraphOf(exec, t).Footprints()[t.Location]
}

// futureOf is the footprint of every move the unit may make from where it
// stands: for a token, the nodes it can reach in its flow and, when its flow
// ends, in the flows it returns to.
func (c *checker) futureOf(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	exec, isAction := m.Owner.(*ActionExecutor)
	if !isAction {
		return lower.Footprint{Dynamic: true}
	}
	return c.tokenFuture(exec, exec.tokens[exec.tokenIndex(m.Token)])
}

func (c *checker) tokenFuture(exec *ActionExecutor, t Token) lower.Footprint {
	if t.body != nil {
		return lower.Footprint{Dynamic: true}
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
		Reads:   append(slices.Clone(f.Reads), g.Reads...),
		Writes:  append(slices.Clone(f.Writes), g.Writes...),
		Sends:   append(slices.Clone(f.Sends), g.Sends...),
		Accepts: append(slices.Clone(f.Accepts), g.Accepts...),
		Control: append(slices.Clone(f.Control), g.Control...),
		Dynamic: f.Dynamic || g.Dynamic,
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
		action, isAction := exec.(*ActionExecutor)
		if !isAction {
			u := tokenKey{owner: exec}
			p.units = append(p.units, u)
			p.standing[u] = lower.Footprint{Dynamic: true}
			p.future[u] = lower.Footprint{Dynamic: true}
			continue
		}
		for _, t := range action.tokens {
			u := tokenKey{owner: exec, id: t.ID}
			p.units = append(p.units, u)
			p.standing[u] = c.standing(action, t)
			p.future[u] = c.tokenFuture(action, t)
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
