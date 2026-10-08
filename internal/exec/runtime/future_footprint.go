package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// futureKey identifies what a token may still do at this instant: its node in its flow.
type futureKey struct {
	graph *lower.ActionGraph
	node  ast.Node
}

// futureFootprints memoizes what executors may still touch at the clock's instant:
// a token's by its node in its flow, a delay by whether it ends the instant; both
// are dropped once the clock moves. A machine's whole graph is kept by graph.
// Once gated, senders are the sends any executor may make at this instant, and
// anySender marks an executor that may send anything.
type futureFootprints struct {
	ctx       *Context
	now       float64
	nodes     map[futureKey]lower.Footprint
	ends      map[*ast.TimeEvent]bool
	machines  map[*lower.StateGraph]lower.Footprint
	gated     bool
	senders   []futureSend
	anySender bool
}

// futureSend is a send an executor run for from may make at this instant.
type futureSend struct {
	channel lower.Channel
	from    *Instance
}

// newFutureFootprints reads the clock of ctx, which bind replaces.
func newFutureFootprints(ctx *Context) *futureFootprints {
	return &futureFootprints{ctx: ctx, machines: make(map[*lower.StateGraph]lower.Footprint)}
}

// bind reads the clock of ctx from now on.
func (f *futureFootprints) bind(ctx *Context) {
	f.ctx, f.nodes = ctx, nil
}

// at drops what was memoized for an instant the clock has left.
func (f *futureFootprints) at() {
	if f.nodes != nil && f.now == f.ctx.clock.now {
		return
	}
	f.now = f.ctx.clock.now
	f.nodes = make(map[futureKey]lower.Footprint)
	f.ends = make(map[*ast.TimeEvent]bool)
}

// endsInstant reports whether waiting on trigger provably outlasts this instant:
// a literal delay whose wait comes due after now. Anything else may not.
func (f *futureFootprints) endsInstant(trigger ast.Node, scope *symbols.Scope) bool {
	t, literal := lower.InstantDelay(trigger)
	if !literal {
		return false
	}
	if ends, ok := f.ends[t]; ok {
		return ends
	}
	ends := false
	if magnitude, ok := f.ctx.literalDelay(scope, t.Duration); ok {
		due, err := f.ctx.dueInstant(t, Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: magnitude}}, triggerDescription(t))
		ends = err == nil && due > f.ctx.clock.now
	}
	f.ends[t] = ends
	return ends
}

// gate gathers what every executor may send at this instant, from its future
// with every message-triggered transition open, so a machine's future leaves out
// a transition no message queued or sendable at this instant can fire.
func (f *futureFootprints) gate() {
	f.at()
	f.gated, f.senders, f.anySender = false, f.senders[:0], len(f.ctx.pendingBehaviors) > 0
	for _, w := range f.ctx.clock.waiters {
		if f.anySender {
			break
		}
		if w.finished() {
			continue
		}
		future := f.executorFuture(w)
		f.anySender = future.Dynamic
		for _, c := range future.Sends {
			f.senders = append(f.senders, futureSend{channel: c, from: executorSelf(w)})
		}
	}
	f.gated = true
}

// cannotFire reports whether no message the transition's accept takes is queued
// or may be sent at this instant, once gated: its trigger cannot occur before the clock moves.
func (f *futureFootprints) cannotFire(e *StateExecutor, trans *lower.Transition) bool {
	accept, ok := trans.Trigger.(*ast.AcceptEvent)
	if !f.gated || f.anySender || !ok {
		return false
	}
	want := ast.AsQualifiedName(accept.SignalType)
	if want == nil {
		return false
	}
	for _, m := range f.ctx.messages {
		if f.ctx.messageMatches(m, want, trans.Scope) {
			return false
		}
	}
	consumer, _ := triggerChannel(trans)
	for _, send := range f.senders {
		if f.ctx.sendMeets(send.channel, send.from, consumer, e.self) {
			return false
		}
	}
	return true
}

// executorFuture is what the executor may still touch at this instant: every
// token's future for an action, the machine's for a state machine, everything
// for any other.
func (f *futureFootprints) executorFuture(w clockWaiter) lower.Footprint {
	f.at()
	switch exec := w.(type) {
	case *ActionExecutor:
		return f.actionFuture(exec)
	case *StateExecutor:
		return f.machineFuture(exec)
	}
	return lower.Footprint{Dynamic: true}
}

func (f *futureFootprints) actionFuture(exec *ActionExecutor) lower.Footprint {
	var future lower.Footprint
	for _, t := range exec.tokens {
		future = unionFootprints(future, f.tokenFuture(exec, t))
	}
	return future
}

// tokenFuture is what the token may still touch at this instant: the nodes it can
// reach from its own over successions before a wait ending the instant, the one
// it stands at included, and those reachable from the nodes its frames stand at.
func (f *futureFootprints) tokenFuture(exec *ActionExecutor, t Token) lower.Footprint {
	f.at()
	if exec.dynamics != nil {
		return dynamicsFootprint()
	}
	graph := tokenGraphOf(exec, t)
	var future lower.Footprint
	if t.Wait != nil && t.Wait.Timed {
		if t.Wait.Due > f.ctx.clock.now {
			return standing(exec, t)
		}
		future = graph.Footprints()[t.Location]
		for _, edge := range graph.Edges[t.Location] {
			future = unionFootprints(future, f.reach(graph, edge.Target))
		}
	} else {
		future = f.reach(graph, t.Location)
	}
	for frame := t.frame; frame != nil && frame.node != nil; frame = frame.parent {
		flow := frame.flow
		if flow == nil && frame.parent != nil {
			flow = frame.parent.graph
		}
		if flow == nil {
			flow = exec.graph
		}
		future = unionFootprints(future, f.reach(flow, frame.node))
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
// graph over its successions, the flows those nodes own included, going on past
// no accept whose delay ends the instant; memoized for the instant.
func (f *futureFootprints) reach(graph *lower.ActionGraph, node ast.Node) lower.Footprint {
	key := futureKey{graph: graph, node: node}
	if fp, ok := f.nodes[key]; ok {
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
		if sub, owns := graph.Subflows[n]; owns {
			if sub == nil || sub.Graph == nil {
				future.Dynamic = true
			} else {
				for _, start := range sub.Graph.Starts() {
					future = unionFootprints(future, f.reach(sub.Graph, start))
				}
			}
		}
		if f.waitEndsInstant(graph, n) {
			continue
		}
		for _, edge := range graph.Edges[n] {
			work = append(work, edge.Target)
		}
	}
	f.nodes[key] = future
	return future
}

// waitEndsInstant reports whether a token reaching n waits there past this instant.
func (f *futureFootprints) waitEndsInstant(graph *lower.ActionGraph, n ast.Node) bool {
	accept, ok := graph.Accepts[n]
	if !ok {
		return false
	}
	scope := accept.Scope
	if scope == nil {
		scope = graph.Scope
	}
	return f.endsInstant(accept.Trigger, scope)
}

// machineFuture is what the machine may still touch at this instant: the
// transitions out of its configuration it may fire, on through the states they
// enter, and its do behaviors from where they stand. A machine not yet started,
// holding an entry or running a call may do anything its graph does.
func (f *futureFootprints) machineFuture(e *StateExecutor) lower.Footprint {
	f.at()
	if e.activeConfig == nil || len(e.held) > 0 || e.pendingCall != nil {
		return withDrop(e, f.wholeMachine(e))
	}
	timerDue := false
	for _, wait := range e.armedWaits() {
		if _, timer := wait.what.(transitionWait); timer && wait.Due <= f.ctx.clock.now {
			timerDue = true
		}
	}
	var future lower.Footprint
	active := make(map[*ast.StateNode]bool)
	var work []*ast.StateNode
	for _, leaf := range e.activeStates() {
		for _, state := range e.getParentChain(leaf) {
			if !active[state] {
				active[state] = true
				work = append(work, state)
			}
		}
	}
	for _, act := range e.doActions {
		future = unionFootprints(future, f.doFuture(e, act))
	}
	steps := e.graph.TransitionSteps()
	seen := make(map[*ast.StateNode]bool)
	for len(work) > 0 {
		state := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[state] {
			continue
		}
		seen[state] = true
		for _, trans := range e.graph.Transitions[state] {
			if !(active[state] && timerDue) && f.endsInstant(trans.Trigger, trans.Scope) || f.cannotFire(e, trans) {
				continue
			}
			step, ok := steps[trans]
			if !ok {
				return withDrop(e, f.wholeMachine(e))
			}
			future = unionFootprints(future, step.Footprint)
			if channel, takes := triggerChannel(trans); takes {
				future.Accepts = append(future.Accepts, channel)
			}
			for _, entered := range step.Enters {
				future = unionFootprints(future, f.startedDo(e, entered))
				work = append(work, entered)
			}
		}
	}
	return withDrop(e, future)
}

// doFuture is what a do behavior under way or pending may still do at this
// instant: a flow it runs from where its tokens stand, else the whole of it.
func (f *futureFootprints) doFuture(e *StateExecutor, act *doAction) lower.Footprint {
	var future lower.Footprint
	if run := act.run; run != nil {
		behavior := run.host.behavior
		if graph, flow := statedFlow(behavior), run.host.flow; graph != nil && flow != nil && flow.graph == graph && run.body.paused.wait.held == nil {
			future = f.actionFuture(flow)
		} else {
			future = e.graph.BehaviorFootprints()[behavior.Node]
		}
	}
	for _, behavior := range act.pending {
		future = unionFootprints(future, f.behaviorFuture(e, behavior))
	}
	return future
}

// startedDo is what the do behaviors entering state starts may do at this instant.
func (f *futureFootprints) startedDo(e *StateExecutor, state *ast.StateNode) lower.Footprint {
	var future lower.Footprint
	if behaviors := e.graph.Behaviors[state]; behaviors != nil {
		for _, behavior := range behaviors.Do {
			future = unionFootprints(future, f.behaviorFuture(e, behavior))
		}
	}
	return future
}

// behaviorFuture is what starting the behavior may do at this instant: a stated
// flow up to the waits ending it, else the whole behavior.
func (f *futureFootprints) behaviorFuture(e *StateExecutor, behavior lower.StateBehavior) lower.Footprint {
	graph := statedFlow(behavior)
	if graph == nil {
		return e.graph.BehaviorFootprints()[behavior.Node]
	}
	var future lower.Footprint
	if behavior.Owner != nil {
		future.Reads = []lower.Place{{Name: behavior.Owner.Name, State: behavior.Owner}}
	}
	for _, start := range graph.Starts() {
		future = unionFootprints(future, f.reach(graph, start))
	}
	return future
}

// statedFlow is the flow a behavior whose body is one block stating a flow runs.
func statedFlow(behavior lower.StateBehavior) *lower.ActionGraph {
	if len(behavior.Body) != 1 {
		return nil
	}
	if block, ok := behavior.Body[0].(lower.Block); ok {
		return block.Graph
	}
	return nil
}

// wholeMachine is the footprint of every move the machine may make from any
// configuration: its transitions' and its behaviors', with the messages any
// trigger of it takes; memoized by graph.
func (f *futureFootprints) wholeMachine(e *StateExecutor) lower.Footprint {
	if fp, ok := f.machines[e.graph]; ok {
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
	f.machines[e.graph] = future
	return future
}

// literalDelay is the magnitude, in clock units, of a delay written as a number
// with or without a time unit, read without evaluating it.
func (ctx *Context) literalDelay(scope *symbols.Scope, duration ast.Node) (float64, bool) {
	var unit ast.Node
	if ix, quantity := duration.(*ast.IndexExpr); quantity && ix.Bracket {
		duration, unit = ix.Operand, ix.Index
	}
	var num semantics.Value
	switch lit := duration.(type) {
	case *ast.LiteralInteger:
		val, ok := semantics.ParseInteger(lit.Value)
		if !ok {
			return 0, false
		}
		num = val
	case *ast.LiteralReal:
		val, err := semantics.ParseReal(lit.Value)
		if err != nil {
			return 0, false
		}
		num = semantics.Value{Kind: semantics.ValReal, Real: val}
	default:
		return 0, false
	}
	if unit == nil {
		return num.AsReal(), true
	}
	term, err := ctx.model.semantics.UnitTermOfExpr(scope, unit)
	if err != nil {
		return 0, false
	}
	product, err := ctx.model.semantics.UnitProductOfExpr(scope, unit)
	if err != nil {
		return 0, false
	}
	q := &Quantity{Num: num, Unit: Unit{Text: semantics.UnitExprText(unit), Product: product, Term: term}}
	magnitude, err := ctx.durationInClockUnits(q, "delay")
	return magnitude, err == nil
}
