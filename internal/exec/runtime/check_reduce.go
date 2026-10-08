package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Static partial-order reduction: from each state the search explores a
// persistent set of the enabled moves, closed under "a unit whose future may
// not commute with a member is a member", less a sleep set of the moves an
// equivalent predecessor already explored (bounded-model-checking.md, "The algorithm").
// A unit is what a move advances: an action's token, or a state machine as a whole.

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
		return standing(exec, exec.tokens[exec.tokenIndex(m.Token)])
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
func standing(exec *ActionExecutor, t Token) lower.Footprint {
	if exec.dynamics != nil {
		return dynamicsFootprint()
	}
	return tokenGraphOf(exec, t).Footprints()[t.Location]
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
		}
	}
	return withDrop(e, fp)
}

// withDrop adds to fp the drop a dispatch of a machine run for an object may make.
func withDrop(e *StateExecutor, fp lower.Footprint) lower.Footprint {
	if e.self != nil {
		fp.Accepts = append(slices.Clip(fp.Accepts), lower.Channel{Drops: true})
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
			channel.Signal, channel.Type, channel.Scope = lower.FeaturePath(typed), typed, trans.Scope
		}
		return channel, true
	case *ast.CallEvent:
		return lower.Channel{Port: trans.Via}, true
	}
	return lower.Channel{}, false
}

// doStepFootprint is what one step of the state's do behavior runs: a move of a
// token of the flow it runs, else the rest of the behavior paused mid-way, covered
// by that behavior's, or the next one pending.
func doStepFootprint(e *StateExecutor, state ast.Node) lower.Footprint {
	for _, act := range e.doActions {
		if act.state != state {
			continue
		}
		if run := act.run; run != nil {
			if flow := runningFlow(run); flow != nil {
				fp := lower.Footprint{Reads: []lower.Place{{Name: run.host.behavior.Owner.Name, State: run.host.behavior.Owner}}}
				for _, t := range flow.tokens {
					fp = unionFootprints(fp, standing(flow, t))
				}
				return fp
			}
			return e.graph.BehaviorFootprints()[run.host.behavior.Node]
		}
		if len(act.pending) == 0 {
			return lower.Footprint{}
		}
		return e.graph.BehaviorFootprints()[act.pending[0].Node]
	}
	return lower.Footprint{}
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
		Creates:    f.Creates || g.Creates,
		Routes:     f.Routes || g.Routes,
	}
}

// dependent reports whether the two moves may not commute: the moves of one
// unit never do, and otherwise their footprints decide.
func (c *checker) dependent(a, b searchMove) bool {
	return a.unit() == b.unit() || a.footprint.DependentBy(b.footprint, c.relation(a.Owner, b.Owner))
}

// relation is what is known of the two executors moves belong to.
func (c *checker) relation(a, b clockWaiter) lower.Relation {
	return c.ctx.relation(a, b)
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
				p.standing[u] = standing(e, t)
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

// include adds a unit: its moves, and the units whose future may let it go on
// or meet any move it may make next, enabled or not.
func (p *persistentClosure) include(u tokenKey) {
	if p.in[u] {
		return
	}
	p.in[u] = true
	standing := p.standing[u]
	for _, other := range p.units {
		if !p.in[other] && p.future[other].DependentBy(standing, p.c.relation(other.owner, u.owner)) {
			p.include(other)
		}
	}
}

// futureDependsOnSet reports whether the unit's future may not commute with
// a move of the set.
func (p *persistentClosure) futureDependsOnSet(u tokenKey) bool {
	future := p.future[u]
	for _, m := range p.all {
		if p.in[m.unit()] && future.DependentBy(m.footprint, p.c.relation(u.owner, m.Owner)) {
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
		if !c.dependent(s, m) {
			sleep = append(sleep, s)
		}
	}
	for _, s := range f.moves[:f.next-1] {
		if !c.dependent(s, m) {
			sleep = append(sleep, s)
		}
	}
	return sleep
}

// runningFlow is the flow a do run steps a token of at a time: that of a behavior
// whose body states one, under way with a token and not held at a wait; nil otherwise.
func runningFlow(run *doRun) *ActionExecutor {
	behavior := run.host.behavior
	graph, flow := statedFlow(behavior), run.host.flow
	if graph == nil || flow == nil || flow.graph != graph || run.body.paused.wait.held != nil || len(flow.tokens) == 0 || behavior.Owner == nil {
		return nil
	}
	return flow
}
