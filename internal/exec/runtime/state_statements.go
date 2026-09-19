package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// stateStmtHost runs the statements of a state machine behavior written as an
// inline action body: an assignment to a name the body does not declare reaches
// the machine's state data, and a send posts through the machine's connections.
// The behavior's execution is a performance whose nodes the body's blocks declare.
type stateStmtHost struct {
	exec     *StateExecutor
	behavior lower.StateBehavior
	// flow runs the token flow the body or one of its nodes states, as the
	// standalone action executor does; perfs are its performances.
	flow  *ActionExecutor
	perfs *performances
	// attrs are the attributes of the state the behavior belongs to and of the
	// states enclosing it, innermost first.
	attrs []map[string]Value
}

// executeBehavior runs one behavior of a state or transition to its end at the
// instant it is triggered: an entry, exit or effect behavior does not wait on the
// clock, so one whose flow does is an error; a do behavior runs as a doRun instead.
func (e *StateExecutor) executeBehavior(behavior lower.StateBehavior) error {
	if len(behavior.Body) == 0 {
		return nil
	}
	host := e.behaviorHost(behavior)
	defer e.ctx.holdClock(host.describe())()
	return host.run()
}

// behaviorHost prepares one execution of a behavior: its performance over the
// machine's data and the attributes of the states around it.
func (e *StateExecutor) behaviorHost(behavior lower.StateBehavior) *stateStmtHost {
	host := &stateStmtHost{exec: e, behavior: behavior, attrs: e.attrFramesFor(behavior.Owner)}
	host.flow = &ActionExecutor{
		performances:     performances{ctx: e.ctx, self: e.self, root: host.rootFrame(host.attrs), owner: host, behavior: e.stateMachine},
		action:           behaviorSymbol(behavior),
		state:            StateRunning,
		nextTokenID:      1,
		breakpoints:      make(map[string]bool),
		firedBreakpoints: make(map[breakpointVisit]bool),
	}
	host.flow.flow = host.flow
	host.flow.driven.exec = host.flow
	host.flow.driven.caller = &e.driven
	host.perfs = &host.flow.performances
	return host
}

// run executes the behavior's statements; a do behavior's pause where they wait
// and are re-entered (perform).
func (h *stateStmtHost) run() error {
	_, err := h.exec.ctx.runStatements(func() *stmtEngine {
		engine := newStmtEngineOver(h.exec.ctx, h, h.exec.stateData, h.attrs)
		engine.env.perf = h.perfs.root
		return engine
	}, h.behavior.Body)
	return h.ended(err)
}

// ended settles a run of the behavior that err ended: a terminate of the behavior's
// own performance is its end, dropping what its flow still ran and ending the
// performances nested in it; any other err is returned as is.
func (h *stateStmtHost) ended(err error) error {
	root := h.perfs.root
	if !terminates(err, root) {
		return err
	}
	if !root.inBody || len(h.flow.tokensIn(root)) > 0 {
		h.flow.dropTokensIn(root, 0)
	}
	endNested(root)
	root.live = 0
	h.flow.state = StateCompleted
	if t := unwound(err); t != nil {
		return h.flow.endAlongside(t)
	}
	return nil
}

// endNested marks the performances nested in perf ended, perf itself kept.
func endNested(perf *actionFrame) {
	for _, sub := range perf.subactions {
		sub.ended, sub.live = true, 0
		endNested(sub)
	}
}

func (h *stateStmtHost) perform() error { return h.run() }

// clone is the host itself: what its run changes is the flow's, captured with it.
func (h *stateStmtHost) clone() bodyWork { return h }

// doRun is a do behavior under way as a body run: it yields after each statement
// of its body, and a wait on the clock or for a message in its flow pauses it
// there, to be resumed in a later round or ended when the state is exited, while
// the machine goes on around it.
type doRun struct {
	host *stateStmtHost
	body *bodyRun
	// mail is what the run's accepts read in place of the bus: the message the
	// machine is dispatching to it, none between dispatches.
	mail []Message
}

// startDoRun begins a do behavior, performing its first statement and pausing it
// there; nil once it has ended, with the error that ended it.
func (e *StateExecutor) startDoRun(behavior lower.StateBehavior) (*doRun, error) {
	if len(behavior.Body) == 0 {
		return nil, nil
	}
	host := e.behaviorHost(behavior)
	body := &bodyRun{work: host, awaitsMessages: true, yields: true}
	run := &doRun{host: host, body: body}
	return run.resume(e.ctx)
}

// resume lets the run go on to its next statement boundary or wait, or to its end.
func (run *doRun) resume(ctx *Context) (*doRun, error) {
	defer ctx.readingMail(&run.mail)()
	defer func() { run.mail = nil }()
	run.host.flow.leftStanding = false
	for {
		pause, paused := run.body.resume(ctx)
		if !paused {
			return nil, run.body.err
		}
		if pause.onWait || pause.yielded {
			return run, nil
		}
	}
}

// offer resumes the run with the message its machine dispatches to it, which the
// accept it is parked at takes; the run goes on to where it next waits.
func (run *doRun) offer(ctx *Context, m Message) (*doRun, error) {
	run.mail = []Message{m}
	return run.resume(ctx)
}

// resumable reports a run due to go on: one yielded between statements, or one
// whose wait on the clock has ended; a run parked for a message stays until its
// machine dispatches one to it.
func (run *doRun) resumable(ctx *Context) bool {
	if run.body.paused.yielded {
		return true
	}
	defer ctx.readingMail(&run.mail)()
	return !run.body.paused.wait.goesOn()
}

// end abandons the run for good: nothing of the behavior runs after it.
func (run *doRun) end(ctx *Context) {
	run.body.end(ctx)
}

// messageAcceptor is an executor a message in flight may let go on from an accept
// it is parked at.
type messageAcceptor interface {
	// acceptTaking lists the accepts parked for m, placed in the performance parked there.
	acceptTaking(m Message) ([]TakingAccept, error)
	// performanceName names the performance the accepts are placed in.
	performanceName() string
}

// acceptsMessage reports whether the run, paused at an accept of its flow or of
// the action it performs, would take m; a port failing to resolve is the error.
func (run *doRun) acceptsMessage(m Message) (bool, error) {
	if accepted, err := run.host.flow.acceptsMessage(m); err != nil || accepted {
		return accepted, err
	}
	if held := run.body.paused.wait.held; held != nil {
		return held.acceptsMessage(m)
	}
	return false, nil
}

// armedWaits lists the waits on the clock the run is paused on, due or not: its
// flow's, and those of the action it performs where that is what waits.
func (run *doRun) armedWaits() []ClockWait {
	waits := run.host.flow.armedWaits()
	if held := run.body.paused.wait.held; held != nil {
		waits = append(waits, held.armedWaits()...)
	}
	return waits
}

// visibleArmedWaits lists armedWaits and the waits of the actions performed, in
// turn, for the paused work of the flow and of the action performed.
func (run *doRun) visibleArmedWaits() []ClockWait {
	waits := run.host.flow.visibleArmedWaits()
	if held := run.body.paused.wait.held; held != nil {
		waits = append(waits, held.visibleArmedWaits()...)
	}
	return waits
}

// rootFrame is the performance of the behavior itself: it holds no feature of its
// own, and reads the machine's data and the enclosing states' attributes around it.
func (h *stateStmtHost) rootFrame(attrs []map[string]Value) *actionFrame {
	root := &actionFrame{
		scope:       h.behavior.Scope,
		connections: h.exec.graph.Connections,
		data:        make(map[string]Value),
		features:    make(map[string]ast.FeatureDirection),
		subactions:  make(map[ast.Node]*actionFrame),
		nodes:       h.behavior.Nodes,
		label:       h.describe(),
		outer:       []frame{mapFrame(h.exec.stateData)},
		run:         h.exec.ctx.newRun(),
	}
	if root.scope == nil {
		root.scope = h.exec.stateMachine.Scope
	}
	for _, attr := range attrs {
		root.outer = append(root.outer, mapFrame(attr))
	}
	return root
}

// behaviorSymbol is the symbol of the action a behavior's inline body declares,
// nil for a behavior written in another form.
func behaviorSymbol(behavior lower.StateBehavior) *symbols.Symbol {
	for _, stmt := range behavior.Body {
		block, ok := stmt.(lower.Block)
		if !ok || block.Scope == nil || block.Scope.Node() != block.Node {
			continue
		}
		return block.Scope.Owner()
	}
	return nil
}

func (h *stateStmtHost) describe() string {
	if h.behavior.Name != "" {
		return "state behavior " + h.behavior.Name
	}
	if usage, ok := h.behavior.Node.(*ast.Usage); ok {
		return "state behavior " + stateActionName(usage)
	}
	return "anonymous state behavior"
}

func (h *stateStmtHost) send(ec *EvalContext, s lower.Send) error {
	return h.exec.ctx.send(ec, h.exec.stateMachine.Scope, h.exec.graph.Connections, s, h.exec.self, h.exec.stateMachine)
}

// assignOuter writes a name the machine does not declare to the object
// exhibiting it, falling back to executor-local data.
func (h *stateStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return err
	}
	if h.exec.declaresAttribute(name) {
		return h.exec.assignAttribute(name, value)
	}
	if written, err := assignPerformerFeature(h.exec.ctx, h.exec.self, s.Scope, name, value); written || err != nil {
		return err
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

func (h *stateStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return err
	}
	if h.exec.declaresAttribute(name) {
		return h.exec.assignAttribute(name, value)
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

// assignChain writes the feature a chained target names on the object its chain
// reaches, which is the machine's state data for no chained target.
func (h *stateStmtHost) assignChain(ec *EvalContext, s lower.Assign, value Value) error {
	return assignThroughChain(ec, h.describe(), s, value)
}

// assignStateAttribute writes an attribute owned by the state running this
// behavior, or by one enclosing it, and reports whether it did. The value
// answers to the attribute's declaration as every other write does.
func (h *stateStmtHost) assignStateAttribute(name string, value Value) (bool, error) {
	data, scope, ok := h.exec.stateAttributeValues(h.behavior.Owner, name)
	if !ok {
		return false, nil
	}
	if err := h.exec.ctx.checkNamedWrite(scope, h.describe(), name, &value); err != nil {
		return true, err
	}
	data[name] = value
	return true, nil
}

// performer is the object exhibiting the machine this behavior belongs to.
func (h *stateStmtHost) performer() *Instance {
	return h.exec.self
}

// acceptReturn rejects a `return`: a state behavior computes no result.
func (h *stateStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s", ErrReturnOutsideCalc, h.describe())
}

// effect performs the action a `perform` names, or ends the performance a
// `terminate` names; every other effect a body may state has no execution in a
// state behavior.
func (h *stateStmtHost) effect(engine *stmtEngine, s lower.Effect) error {
	if s.Kind == lower.EffectTerminate {
		return h.perfs.terminate(engine, h.perfs.root, s)
	}
	if s.Kind == lower.EffectPerform {
		inv, ok := performedInvocation(s)
		if !ok {
			return fmt.Errorf("%s performs no action", h.describe())
		}
		return h.exec.invokeNested(inv)
	}
	return fmt.Errorf("%s: '%s' in a body is not executable", h.describe(), s.Kind)
}

// performNode performs a nested action of a block as a subperformance of the
// behavior's, in a frame of its own (performances.performNode).
func (h *stateStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	return h.perfs.performNode(h.perfs.root, engine, graph, node)
}

// runFlow runs the token flow an inline body states with its successions and
// control nodes, as the behavior's own performance: the body's attributes are
// the performance's, initialized as a standalone action's are.
func (h *stateStmtHost) runFlow(block lower.Block) (stmtFlow, error) {
	if resumingAt[*subflowFrame](h.exec.ctx) {
		return flowNext, h.flow.runSubflow(h.flow.root)
	}
	if block.Graph.Initial == nil {
		return flowNext, fmt.Errorf("%w: %s: no node starts the flow%s",
			ErrInvalidActionFlow, h.describe(), noFlowStart(block.Graph))
	}
	if err := h.flow.validateSubflows(block.Graph); err != nil {
		return flowNext, fmt.Errorf("%s: %w", h.describe(), err)
	}
	h.flow.graph = block.Graph
	if err := h.flow.checkResultParameters(); err != nil {
		return flowNext, fmt.Errorf("%s: %w", h.describe(), err)
	}
	root := h.flow.root
	h.flow.features = h.flow.performanceFeatures()
	root.graph = block.Graph
	root.connections = block.Graph.Connections
	root.live = 1
	if block.Graph.Scope != nil {
		root.scope = block.Graph.Scope
	}
	h.flow.declareRootFeatures(root)
	h.flow.declareAcceptPayloads(root)
	if err := h.flow.initializeAttributes(); err != nil {
		return flowNext, fmt.Errorf("%s: %w", h.describe(), err)
	}
	return flowNext, h.flow.runSubflow(root)
}

// setFeature writes a feature the behavior's performance holds: an attribute of
// the body's flow, else what is around it.
func (h *stateStmtHost) setFeature(name string, value Value) error {
	if root := h.perfs.root; root.holds(name) {
		if err := h.exec.ctx.checkNamedWrite(root.scope, h.describe(), name, &value); err != nil {
			return err
		}
		root.data[root.key(name)] = value
		return nil
	}
	if written, err := h.assignAround(name, value); written || err != nil {
		return err
	}
	return fmt.Errorf("%w: %s holds no %s", ErrBindingEnd, h.describe(), name)
}

// assignAround writes what a node's performance returns to the state's attribute,
// the machine's attribute or the state datum of that name, whichever exists.
func (h *stateStmtHost) assignAround(name string, value Value) (bool, error) {
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return written, err
	}
	if h.exec.declaresAttribute(name) {
		return true, h.exec.assignAttribute(name, value)
	}
	if _, ok := h.exec.stateData[name]; ok {
		h.exec.stateData[name] = value
		return true, nil
	}
	return assignPerformerFeature(h.exec.ctx, h.exec.self, h.behavior.Scope, name, value)
}

// pauseAt sets no breakpoint: a state behavior's nodes are not stepped.
func (h *stateStmtHost) pauseAt([]ast.Node, ast.Node) error {
	return nil
}

// runOwnFlow runs the flow a nested node states of its own, as a subflow of the
// behavior's performance.
func (h *stateStmtHost) runOwnFlow(perf *actionFrame) error {
	return h.flow.runSubflow(perf)
}

// endsOwn allows a terminate to end the behavior's own performance (SysML v2
// §7.17.10): the behavior ends at the statement and the state it belongs to stays.
func (h *stateStmtHost) endsOwn() bool { return true }

// performedInvocation reports the action a `perform` statement declared in scope
// names, in either form the parser produces for one.
func performedInvocation(s lower.Effect) (actionInvocation, bool) {
	inv, ok := statementInvocation(s.Node)
	if ok {
		inv.step = memberSymbol(s.Scope, s.Node)
	}
	return inv, ok
}

// statementInvocation reads the action a `perform` statement names.
func statementInvocation(node ast.Node) (actionInvocation, bool) {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		if inv := n.PerformedInvocation(); inv != nil {
			return expressionInvocation(inv), true
		}
		switch ref := n.ActionRef.(type) {
		case *ast.QualifiedName:
			return actionInvocation{target: ref}, true
		case *ast.FeatureChainExpr:
			return chainedInvocation(ref, nil)
		}
	case *ast.Usage:
		return nestedInvocation(n)
	case *ast.ActionExecutionNode:
		if n.ActionRef != nil {
			return actionInvocation{target: n.ActionRef}, true
		}
	}
	return actionInvocation{}, false
}

// declaredOutput reports no output features: a state behavior computes none.
func (h *stateStmtHost) declaredOutput(string) bool {
	return false
}
