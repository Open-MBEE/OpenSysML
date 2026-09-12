package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// Static partial-order reduction: from each state the search explores a
// persistent set of the enabled moves, closed under "a token whose future may
// not commute with a member is a member", less a sleep set of the moves an
// equivalent predecessor already explored (bounded-model-checking.md, "The algorithm").

// futureKey identifies what a token may still do: its node in its flow.
type futureKey struct {
	graph *lower.ActionGraph
	node  ast.Node
}

// footprintOf is the footprint of the move: what advancing the token over its
// node touches. A move the executor will refuse depends on everything.
func (c *checker) footprintOf(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	return c.standing(c.exec.tokens[c.exec.tokenIndex(m.Token)])
}

// standing is the footprint of the node the token stands at.
func (c *checker) standing(t Token) lower.Footprint {
	if t.body != nil {
		return lower.Footprint{Dynamic: true}
	}
	return c.tokenGraphOf(t).Footprints[t.Location]
}

// futureOf is the footprint of every move the token may make from where it
// stands: the nodes it can reach in its flow and, when its flow ends, in the
// flows it returns to.
func (c *checker) futureOf(m enabledMove) lower.Footprint {
	if m.Fails != nil {
		return lower.Footprint{Dynamic: true}
	}
	return c.tokenFuture(c.exec.tokens[c.exec.tokenIndex(m.Token)])
}

func (c *checker) tokenFuture(t Token) lower.Footprint {
	if t.body != nil {
		return lower.Footprint{Dynamic: true}
	}
	future := c.reach(c.tokenGraphOf(t), t.Location)
	for frame := t.frame; frame != nil && frame.node != nil; frame = frame.parent {
		flow := frame.flow
		if flow == nil && frame.parent != nil {
			flow = frame.parent.graph
		}
		if flow == nil {
			flow = c.exec.graph
		}
		future = unionFootprints(future, c.reach(flow, frame.node))
	}
	return future
}

func (c *checker) tokenGraphOf(t Token) *lower.ActionGraph {
	if t.frame != nil && t.frame.graph != nil {
		return t.frame.graph
	}
	return c.exec.graph
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
		future = unionFootprints(future, graph.Footprints[n])
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
// token never do, and otherwise their footprints decide.
func dependent(a, b searchMove) bool {
	return a.Token == b.Token || a.footprint.Dependent(b.footprint)
}

// persistent selects the moves to explore from a state, less the moves asleep;
// every move when unreduced or under a property, which reads what no footprint
// names. The set is grown from the first awake move: a
// token whose future may not commute with a member's move joins with every move
// it has; one with none (parked, or waiting at a join) brings in the tokens
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
	closure.include(all[first].Token)
	for grew := true; grew; {
		grew = false
		for _, t := range closure.tokens {
			if closure.in[t.ID] {
				continue
			}
			if closure.futureDependsOnSet(t) {
				closure.include(t.ID)
				grew = true
			}
		}
	}
	// In canonical order, awake members only.
	out := make([]searchMove, 0, len(all))
	for _, m := range all {
		if closure.in[m.Token] && awake(m) {
			out = append(out, m)
		}
	}
	return out
}

// persistentClosure is a persistent set under construction: the tokens in it,
// the moves of the state and the footprints of every token of the state.
type persistentClosure struct {
	c        *checker
	all      []searchMove
	tokens   []Token
	in       map[int64]bool
	standing map[int64]lower.Footprint
	future   map[int64]lower.Footprint
}

func newPersistentClosure(c *checker, all []searchMove) *persistentClosure {
	p := &persistentClosure{
		c:        c,
		all:      all,
		tokens:   c.exec.tokens,
		in:       make(map[int64]bool),
		standing: make(map[int64]lower.Footprint, len(c.exec.tokens)),
		future:   make(map[int64]lower.Footprint, len(c.exec.tokens)),
	}
	for _, t := range c.exec.tokens {
		p.standing[t.ID] = c.standing(t)
		p.future[t.ID] = c.tokenFuture(t)
	}
	for _, m := range all {
		if m.Fails != nil {
			p.standing[m.Token] = lower.Footprint{Dynamic: true}
			p.future[m.Token] = lower.Footprint{Dynamic: true}
		}
	}
	return p
}

// include adds a token: its moves when it has some, else the tokens whose
// future may let it go on.
func (p *persistentClosure) include(token int64) {
	if p.in[token] {
		return
	}
	p.in[token] = true
	if slices.ContainsFunc(p.all, func(m searchMove) bool { return m.Token == token }) {
		return
	}
	standing := p.standing[token]
	for _, t := range p.tokens {
		if !p.in[t.ID] && p.future[t.ID].Dependent(standing) {
			p.include(t.ID)
		}
	}
}

// futureDependsOnSet reports whether the token's future may not commute with
// a move of the set.
func (p *persistentClosure) futureDependsOnSet(t Token) bool {
	future := p.future[t.ID]
	for _, m := range p.all {
		if p.in[m.Token] && future.Dependent(m.footprint) {
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
