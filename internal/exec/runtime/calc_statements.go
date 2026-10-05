package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// calcStmtHost runs a calculation body's statements: it owns its locals, its
// output features and its returned value, and rejects every outside effect.
// A case body (an analysis) is an action's: its steps run as subperformances of
// the case, which flow steps the tokens of; a calc's body performs nothing.
type calcStmtHost struct {
	ctx    *Context
	shape  *calcShape
	self   *Instance
	result Value // the value its `return` yielded
	flow   *ActionExecutor
	perfs  *performances
	env    *stmtEnv // the body's values, which a step's outputs return to
	// occ shares the performance instance `this` denotes with the
	// invocation's evaluation context: nil until the first read materializes it.
	occ *calcOccurrence
}

// calcOccurrence is the performance instance a calc def invocation's `this`
// denotes, materialized lazily and shared between the invocation's evaluation
// context and its statement host so both answer with the one instance.
type calcOccurrence struct {
	inst        *Instance
	materialize func() (*Instance, error)
	ended       bool
	// params is the run's parameter bindings as they are bound, which a
	// materialization seeds from; its vars map is shared with the run's frame.
	params frame
	// this is materializeOccurrence bound once, which every binding env reads.
	this func() (*Instance, error)
}

// thisOccurrence is materializeOccurrence as the hook an EvalContext holds.
func (o *calcOccurrence) thisOccurrence() func() (*Instance, error) {
	if o == nil {
		return o.materializeOccurrence
	}
	if o.this == nil {
		o.this = o.materializeOccurrence
	}
	return o.this
}

// materializeOccurrence is the invocation's occurrence, made on the first call.
func (o *calcOccurrence) materializeOccurrence() (*Instance, error) {
	if o == nil || o.materialize == nil {
		return nil, nil
	}
	return o.materialize()
}

// attachPerformances makes the host perform the steps of a case body as
// subperformances of the case, whose performance is the root the body reads through.
func (h *calcStmtHost) attachPerformances(engine *stmtEngine) {
	if !h.shape.performs() {
		return
	}
	root := &actionFrame{
		scope:      h.shape.bodyScope(),
		data:       make(map[string]Value),
		features:   make(map[string]ast.FeatureDirection),
		subactions: make(map[ast.Node]*actionFrame),
		nodes:      h.shape.Nodes,
		label:      h.shape.Label,
		outer:      append(append([]frame{}, engine.env.enclosing...), engine.env.data),
		run:        h.ctx.newRun(),
	}
	h.flow = &ActionExecutor{
		performances:     performances{ctx: h.ctx, self: h.self, root: root, owner: h, behavior: h.shape.Sym},
		action:           h.shape.Sym,
		state:            StateRunning,
		nextTokenID:      1,
		breakpoints:      make(map[string]bool),
		firedBreakpoints: make(map[breakpointVisit]bool),
	}
	h.flow.flow = h.flow
	root.perfs = &h.flow.performances
	h.perfs = &h.flow.performances
	h.env = engine.env
	engine.env.perf = root
}

// readPerformance makes the host read the steps a completed performance of the case
// holds, performing none of its own: the results of a body concluded after its runs.
func (h *calcStmtHost) readPerformance(engine *stmtEngine, perf *actionFrame) {
	h.env = engine.env
	engine.env.perf = perf
}

// performance is the case's own performance, nil for a calculation.
func (h *calcStmtHost) performance() *actionFrame {
	if h.perfs == nil {
		return nil
	}
	return h.perfs.root
}

// calcBodyDescription names a calculation body in a diagnostic; the invocation
// adds the calc's name.
const calcBodyDescription = "calculation body"

// describe names the body in a diagnostic.
func (h *calcStmtHost) describe() string {
	return calcBodyDescription
}

func (h *calcStmtHost) send(*EvalContext, lower.Send) error {
	return fmt.Errorf("%w: a calculation cannot send a message", ErrCalcSideEffect)
}

// declaredOutput reports whether name is an output the body binds by assigning
// it. An `inout` is bound by the invocation, so writing it writes a parameter.
func (h *calcStmtHost) declaredOutput(name string) bool {
	if h.shape == nil {
		return false
	}
	out, ok := h.shape.output(name)
	return ok && !out.IsInOut
}

// assignOuter binds an output this calculation declares, and rejects any other
// undeclared name: writing that would be an effect outside the calculation.
func (h *calcStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if !h.declaredOutput(name) {
		return fmt.Errorf("%w: %s is not declared by the calculation", ErrCalcExternalAssignment, name)
	}
	out, _ := h.shape.output(name)
	if out.Value != nil && !out.IsInitial {
		return fmt.Errorf(
			"%w: output %s of %s is both given a value by its declaration and assigned in its body",
			ErrConflictingOutput, name, h.shape.Label,
		)
	}
	// Written to the body's own data, so later statements read the output bound —
	// an assignment may accumulate into it — and the read that follows the
	// activation answers from what the body left.
	if err := storeBodyValue(h.ctx, h, env, name, value, s); err != nil {
		return err
	}
	return h.mirrorOccurrence(name, value)
}

func (h *calcStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if err := storeBodyValue(h.ctx, h, env, name, value, s); err != nil {
		return err
	}
	return h.mirrorOccurrence(name, value)
}

// mirrorOccurrence carries a write to a declared feature into the occurrence
// `this` materialized for, as bindCalcParameters mirrors bound inputs.
// calcFeatureWriter is a calc run's frame write path for a feature its bindings
// hold: a qualified write lands the way the body's own assignments do — checked
// against the declaration, bound in the frame, and mirrored into the run's
// occurrence when `this` materialized and holds the feature.
func calcFeatureWriter(ctx *Context, shape *calcShape, occ *calcOccurrence) func(frame, string, Value) error {
	return func(f frame, name string, value Value) error {
		if err := ctx.checkNamedWrite(shape.bodyScope(), calcBodyDescription, name, &value); err != nil {
			return err
		}
		f.set(name, value)
		if occ == nil || occ.inst == nil {
			return nil
		}
		if _, ok := occ.inst.FeatureValues[name]; !ok {
			return nil
		}
		return occ.inst.BindFeatureValue(ctx, name, value)
	}
}

func (h *calcStmtHost) mirrorOccurrence(name string, value Value) error {
	if h.occ != nil && h.occ.inst != nil {
		if _, ok := h.occ.inst.FeatureValues[name]; ok {
			return h.occ.inst.BindFeatureValue(h.ctx, name, value)
		}
	}
	return nil
}

// assignChain rejects a chained target: writing a feature of another object is
// an effect outside the calculation, as writing an undeclared name is.
func (h *calcStmtHost) assignChain(_ *EvalContext, s lower.Assign, _ Value) error {
	return fmt.Errorf("%w: %s writes a feature of another object", ErrCalcExternalAssignment, s.Chain.Text)
}

// assignForeign rejects a qualified write naming an object outside the calc's
// own run for the same reason a chained target is rejected.
func (h *calcStmtHost) assignForeign(_ *EvalContext, s lower.Assign, _ Value) error {
	return fmt.Errorf("%w: %s::%s writes a feature of another object",
		ErrCalcExternalAssignment, s.Owner.Name, s.Target)
}

// acceptReturn takes the value a `return` yields, which the result parameter
// then holds, so it answers to that parameter's declaration.
func (h *calcStmtHost) statementOrder(stmts []lower.Statement) *lower.StatementOrder {
	if h.shape.performs() || len(stmts) < 2 {
		return nil
	}
	key := &stmts[0]
	if order, ok := h.shape.statementOrders.Load(key); ok {
		order := order.(*lower.StatementOrder)
		if h.ctx.scheduling().ordersStatements() || order.HasReversePrecedence() || order.HasSkipped() {
			return order
		}
		return nil
	}
	order := lower.CalcBodyStatementOrder(h.shape.bodyScope(), nil, stmts)
	actual, _ := h.shape.statementOrders.LoadOrStore(key, order)
	order = actual.(*lower.StatementOrder)
	if h.ctx.scheduling().ordersStatements() || order.HasReversePrecedence() || order.HasSkipped() {
		return order
	}
	return nil
}

func (h *calcStmtHost) orderStep() int {
	if h.flow != nil {
		return h.flow.stepCount + 1
	}
	return h.ctx.enclosingExecutorStep()
}

func (h *calcStmtHost) yieldsBetweenStatements() bool { return false }

func (h *calcStmtHost) acceptReturn(value Value, _ lower.Return) error {
	if out := h.shape.resultOutput(); out != nil {
		if err := out.Decl.check(h.ctx, &value, func() string { return "result" }); err != nil {
			return err
		}
	}
	h.result = value
	return nil
}

func (h *calcStmtHost) performer() *Instance {
	// A calculation's steps see the object in its evaluation context, or nil without one.
	return h.self
}

// occurrence is the instance the invocation materialized for `this`, nil
// until a first read makes one; a calc usage materializes none.
func (h *calcStmtHost) occurrence() *Instance {
	if h.occ == nil {
		return nil
	}
	return h.occ.inst
}

// materializeOccurrence makes the occurrence the invocation's `this` denotes.
func (h *calcStmtHost) materializeOccurrence() (*Instance, error) {
	return h.occ.materializeOccurrence()
}

// effect performs the action a `perform` in a case body names, its outputs
// returning to the body's values; a calculation states no effect at all.
func (h *calcStmtHost) effect(_ *stmtEngine, s lower.Effect) error {
	if h.perfs == nil || s.Kind != lower.EffectPerform {
		return fmt.Errorf("%w: a calculation cannot state '%s'", ErrCalcSideEffect, s.Kind)
	}
	inv, ok := performedInvocation(s)
	if !ok {
		return fmt.Errorf("%s: 'perform' names no action to perform", h.describe())
	}
	_, outputs, err := invokeAction(h.ctx, s.Scope, inv, h.env.values(), h.self)
	if err != nil {
		return fmt.Errorf("%s: %w", h.describe(), err)
	}
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !h.env.assign(name, outputs[name]) {
			h.env.data.set(name, outputs[name])
		}
	}
	return nil
}

// performNode performs a nested action of a case body as a subperformance of the
// case (performances.performNode). A calculation keeps no performances: its nested
// action runs in a frame of the body, and performing an action, a flow of its own or
// a connector at its pins is rejected.
func (h *calcStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	if h.perfs != nil {
		return h.perfs.performNode(h.perfs.root, engine, graph, node)
	}
	if _, performs := nestedInvocationInGraph(graph, node); performs {
		return flowNext, fmt.Errorf("%w: a calculation cannot perform action %s", ErrCalcSideEffect, ActionNodeName(node))
	}
	if connectsPins(graph, node) {
		return flowNext, fmt.Errorf("%s: a binding or flow at a pin of %s in a body is not executable",
			h.describe(), nodeDescription(node))
	}
	var nestedSteps []lower.Statement
	var nestedOrder *lower.StatementOrder
	if sub, owns := graph.Subflows[node]; owns && sub != nil {
		var simple bool
		nestedSteps, simple = sub.Graph.StatementList()
		if !simple {
			return flowNext, fmt.Errorf("%s: the flow %s states of its own in a body is not executable",
				h.describe(), nodeDescription(node))
		}
		nestedOrder = graph.StatementOrders[node]
	}
	engine.env.enter()
	defer engine.env.leave()
	defer engine.enterActivation()()
	for _, feature := range graph.Features[node] {
		if feature.Value == nil {
			engine.env.declareUnvalued(feature.Name)
			continue
		}
		value, err := engine.evalIn(feature.Scope).Eval(feature.Value)
		if err != nil {
			return flowNext, fmt.Errorf("eval %s of %s: %w", feature.Name, nodeDescription(node), err)
		}
		engine.env.declare(feature.Name, value)
	}
	if nestedSteps != nil {
		return engine.runWithOrder(nestedSteps, nestedOrder)
	}
	return engine.run(graph.Bodies[node])
}

func (h *calcStmtHost) runBlockFlow(engine *stmtEngine, block lower.Block) (stmtFlow, error) {
	if h.perfs != nil {
		return h.perfs.performBlockFlow(h.perfs.root, engine, block)
	}
	if block.Stated {
		if steps, ok := block.Graph.StatementList(); ok {
			return engine.runWithOrder(steps, block.Order)
		}
	}
	return flowNext, fmt.Errorf("%w: %s: the flow a body states in a calculation is not executable",
		ErrStatementNotExecutable, h.describe())
}

// setFeature writes a feature the case's performance holds; it holds none of its
// own, so the write reaches the body's values.
func (h *calcStmtHost) setFeature(name string, value Value) error {
	if written, err := h.assignAround(name, value); written || err != nil {
		return err
	}
	return fmt.Errorf("%w: %s holds no %s", ErrBindingEnd, h.describe(), name)
}

// assignAround writes what a step's performance returns to the same-named value
// of the body: an output the case declares, or a parameter or local it holds.
func (h *calcStmtHost) assignAround(name string, value Value) (bool, error) {
	if h.env.assignLocal(name, value) {
		return true, nil
	}
	if h.declaredOutput(name) || h.env.data.has(name) {
		if err := h.ctx.checkNamedWrite(h.shape.bodyScope(), h.describe(), name, &value); err != nil {
			return true, err
		}
		h.env.data.set(name, value)
		return true, h.mirrorOccurrence(name, value)
	}
	return false, nil
}

// returnAround writes a returned output as assignAround does: a case has no caller to keep it for.
func (h *calcStmtHost) returnAround(name string, value Value) (bool, error) {
	if h.declaredOutput(name) || h.env.data.has(name) {
		what := func() string { return fmt.Sprintf("%s: output %s returned", h.describe(), name) }
		if err := h.ctx.checkMutable(h.shape.bodyScope(), what, name); err != nil {
			return true, err
		}
	}
	return h.assignAround(name, value)
}

// pauseAt sets no breakpoint: a case's steps are not stepped interactively.
func (h *calcStmtHost) pauseAt([]ast.Node, ast.Node) error {
	return nil
}

// runOwnFlow runs the token flow a step of the case states of its own to completion.
func (h *calcStmtHost) runOwnFlow(perf *actionFrame) error {
	return h.flow.runSubflow(perf)
}

// endsOwn refuses a terminate of the case's own performance: a case runs to its result.
func (h *calcStmtHost) endsOwn() bool { return false }

// runFlow runs the token flow a case body states with its successions and control
// nodes, as the case's own performance; a calculation states none.
func (h *calcStmtHost) runFlow(block lower.Block) (stmtFlow, error) {
	if h.flow == nil {
		return flowNext, fmt.Errorf("%w: %s: a flow of steps in a body is not executable",
			ErrStatementNotExecutable, h.describe())
	}
	if err := lower.FlowStartError(block.Graph); err != nil {
		return flowNext, fmt.Errorf("%w: %s: %w", ErrInvalidActionFlow, h.describe(), err)
	}
	root := h.flow.root
	h.flow.graph = block.Graph
	root.graph = block.Graph
	root.connections = block.Graph.Connections
	if err := h.flow.runSubflow(root); err != nil {
		return flowNext, err
	}
	return flowNext, nil
}

// connectsPins reports whether graph states a binding or flow at a pin of node.
func connectsPins(graph *lower.ActionGraph, node ast.Node) bool {
	for _, binding := range graph.Bindings {
		if binding.Node == node {
			return true
		}
	}
	if len(graph.DataFlows[node]) > 0 {
		return true
	}
	for _, flows := range graph.DataFlows {
		for _, flow := range flows {
			if flow.Target == node {
				return true
			}
		}
	}
	return false
}
