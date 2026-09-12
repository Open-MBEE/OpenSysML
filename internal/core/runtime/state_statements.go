package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
		performances:     performances{ctx: e.ctx, self: e.self, root: host.rootFrame(host.attrs), owner: host},
		action:           behaviorSymbol(behavior),
		state:            StateRunning,
		nextTokenID:      1,
		breakpoints:      make(map[string]bool),
		firedBreakpoints: make(map[breakpointVisit]bool),
	}
	host.perfs = &host.flow.performances
	return host
}

// run executes the behavior's statements.
func (h *stateStmtHost) run() error {
	engine := newStmtEngineOver(h.exec.ctx, h, h.exec.stateData, h.attrs)
	engine.env.perf = h.perfs.root
	// The activation ends with this execution of the body, so a behavior run
	// again does not read what an earlier execution computed.
	defer engine.finish()
	_, err := engine.run(h.behavior.Body)
	return err
}

// doRun is a do behavior under way on a body coroutine: a wait on the clock or
// for a message in its flow pauses it there, to be resumed once the wait ends or
// ended when the state is exited, while the machine goes on around it.
type doRun struct {
	host *stateStmtHost
	body *bodyRun
	// mail is what the run's accepts read in place of the bus: the message the
	// machine is dispatching to it, none between dispatches.
	mail []Message
}

// startDoRun begins a do behavior, pausing it where it first waits; nil once it
// has ended, with the error that ended it.
func (e *StateExecutor) startDoRun(behavior lower.StateBehavior) (*doRun, error) {
	if len(behavior.Body) == 0 {
		return nil, nil
	}
	host := e.behaviorHost(behavior)
	body := &bodyRun{co: e.ctx.takeBodyCoroutine(), work: host.run, awaitsMessages: true}
	run := &doRun{host: host, body: body}
	return run.resume(e.ctx)
}

// resume lets the run go on to where it next waits, or to its end.
func (run *doRun) resume(ctx *Context) (*doRun, error) {
	defer ctx.readingMail(&run.mail)()
	defer func() { run.mail = nil }()
	for {
		pause, paused := run.body.resume(ctx)
		if !paused {
			return nil, run.body.err
		}
		if pause.onWait {
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

// resumable reports a run whose wait on the clock has ended: a run parked for a
// message stays until its machine dispatches one to it.
func (run *doRun) resumable(ctx *Context) bool {
	defer ctx.readingMail(&run.mail)()
	return !run.body.paused.waits()
}

// end abandons the run for good: nothing of the behavior runs after it.
func (run *doRun) end(ctx *Context) {
	run.body.end(ctx)
}

// messageAcceptor is an executor a message in flight may let go on from an accept
// it is parked at.
type messageAcceptor interface {
	acceptsMessage(m Message) (bool, error)
}

// acceptsMessage reports whether the run, paused at an accept of its flow or of
// the action it performs, would take m; a port failing to resolve is the error.
func (run *doRun) acceptsMessage(m Message) (bool, error) {
	if accepted, err := run.host.flow.acceptsMessage(m); err != nil || accepted {
		return accepted, err
	}
	if held, ok := run.body.paused.held.(messageAcceptor); ok {
		return held.acceptsMessage(m)
	}
	return false, nil
}

// armedWaits lists the waits on the clock the run is paused on, due or not: its
// flow's, and those of the action it performs where that is what waits.
func (run *doRun) armedWaits() []ClockWait {
	waits := run.host.flow.armedWaits()
	if held := run.body.paused.held; held != nil {
		waits = append(waits, held.clockWaits()...)
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
	return h.exec.ctx.send(ec, h.exec.stateMachine.Scope, h.exec.graph.Connections, s, h.exec.self)
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

// effect performs the action a `perform` names; every other effect a body may
// state has no execution in a state behavior.
func (h *stateStmtHost) effect(s lower.Effect) error {
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
func (h *stateStmtHost) pauseAt(ast.Node) error {
	return nil
}

// runOwnFlow runs the flow a nested node states of its own, as a subflow of the
// behavior's performance.
func (h *stateStmtHost) runOwnFlow(perf *actionFrame) error {
	return h.flow.runSubflow(perf)
}

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
		if qn, ok := n.ActionRef.(*ast.QualifiedName); ok {
			return actionInvocation{target: qn}, true
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
