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
}

// executeBehavior runs one behavior of a state or transition: the statements it
// was lowered into, whichever form it was written in.
func (e *StateExecutor) executeBehavior(behavior lower.StateBehavior) error {
	if len(behavior.Body) == 0 {
		return nil
	}
	host := &stateStmtHost{exec: e, behavior: behavior}
	attrs := e.attrFramesFor(behavior.Owner)
	host.flow = &ActionExecutor{
		performances:     performances{ctx: e.ctx, self: e.self, root: host.rootFrame(attrs), owner: host},
		action:           behaviorSymbol(behavior),
		state:            StateRunning,
		nextTokenID:      1,
		breakpoints:      make(map[string]bool),
		firedBreakpoints: make(map[breakpointVisit]bool),
	}
	host.perfs = &host.flow.performances
	engine := newStmtEngineOver(e.ctx, host, e.stateData, attrs)
	engine.env.perf = host.perfs.root
	// The activation ends with this execution of the body, so a behavior run
	// again does not read what an earlier execution computed.
	defer engine.finish()
	_, err := engine.run(behavior.Body)
	return err
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
		inv, ok := performedInvocation(s.Node)
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
		return flowNext, fmt.Errorf("%w: %s: no node starts the flow", ErrInvalidActionFlow, h.describe())
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
	if err := h.flow.initializeAttributes(); err != nil {
		return flowNext, fmt.Errorf("%s: %w", h.describe(), err)
	}
	return flowNext, h.flow.runSubflow(root)
}

// setFeature writes a feature the behavior's performance holds: an attribute of
// the body's flow, else what is around it.
func (h *stateStmtHost) setFeature(name string, value Value) error {
	if root := h.perfs.root; root.holds(name) {
		if err := h.exec.ctx.checkNamedWrite(root.scope, h.describe(), name, value); err != nil {
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
	return false, nil
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

// performedInvocation reports the action a `perform` statement names, in either
// form the parser produces for one.
func performedInvocation(node ast.Node) (actionInvocation, bool) {
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
