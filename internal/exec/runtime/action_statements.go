package runtime

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// actionStmtHost runs an action node's body statements: a send posts through the flow's
// connections, and an assignment to an undeclared name reaches the enclosing performances.
type actionStmtHost struct {
	exec *performances
	node ast.Node // the action node whose body is running, for diagnostics
	// perf is the performance the body runs in, whose features it declares into.
	perf *actionFrame
	// graph is the flow node is a node of.
	graph *lower.ActionGraph
	// step is the performance of a statement node's step, owning what its body performs.
	step *actionFrame
}

// executeBody runs the lowered statements graph records for node in perf, the
// performance they belong to, with the performances around it in lexical reach.
func (e *performances) executeBody(perf *actionFrame, graph *lower.ActionGraph, node ast.Node) error {
	_, err := e.ctx.runStatements(func() *stmtEngine {
		host := &actionStmtHost{exec: e, node: node, perf: perf, graph: graph}
		lexical := perf.lexicalFrames()
		return newStmtEngineIn(e.ctx, host, lexical[len(lexical)-1], lexical[:len(lexical)-1])
	}, graph.Bodies[node])
	return err
}

// executeStatementBody runs the statements of the statement node step performs, in the
// performance around it, with what they perform owned by step.
func (e *performances) executeStatementBody(step *actionFrame, graph *lower.ActionGraph) error {
	perf := step.parent
	_, err := e.ctx.runStatements(func() *stmtEngine {
		host := &actionStmtHost{exec: e, node: step.node, perf: perf, graph: graph, step: step}
		lexical := perf.lexicalFrames()
		return newStmtEngineIn(e.ctx, host, lexical[len(lexical)-1], lexical[:len(lexical)-1])
	}, graph.Bodies[step.node])
	return err
}

// around is the performance the body's nodes are performed in and its terminates resolve from.
func (h *actionStmtHost) around() *actionFrame {
	if h.step != nil {
		return h.step
	}
	return h.perf
}

// runNodeBody runs the statements a control or initial node's body declares,
// which the token passing through the node performs.
func (e *ActionExecutor) runNodeBody(frame *actionFrame, node ast.Node) error {
	if len(frame.graph.Bodies[node]) == 0 {
		return nil
	}
	return e.executeBody(frame, frame.graph, node)
}

func (h *actionStmtHost) describe() string {
	return "action node " + ActionNodeName(h.node)
}

func (h *actionStmtHost) send(ec *EvalContext, s lower.Send) error {
	return h.exec.ctx.send(ec, h.exec.root.scope, h.perf.connections, s, h.exec.self, h.exec.behavior)
}

// assignOuter writes a name the body's blocks do not declare: to the running performance,
// else the innermost enclosing one holding it, else the performing object, else the body.
func (h *actionStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
	name = h.perf.heldKey(h.exec.ctx, s.Scope, name)
	if h.perf.declares(name) {
		return h.exec.setFrameFeature(h.perf, name, value)
	}
	if written, err := h.exec.assignEnclosing(h.perf, name, value); written || err != nil {
		return err
	}
	if written, err := assignPerformerFeature(h.exec.ctx, h.exec.self, s.Scope, name, value); written || err != nil {
		return err
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

func (h *actionStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	name = h.perf.heldKey(h.exec.ctx, s.Scope, name)
	if h.perf.declares(name) {
		return h.exec.setFrameFeature(h.perf, name, value)
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

// assignChain writes the feature a chained target names on the object its chain
// reaches from where the statement was written.
func (h *actionStmtHost) assignChain(ec *EvalContext, s lower.Assign, value Value) error {
	return assignThroughChain(ec, h.describe(), s, value)
}

// assignForeign writes a qualified target naming a feature outside the body's
// own run: an enclosing run's frame, else the performing object the qualifier types.
func (h *actionStmtHost) assignForeign(ec *EvalContext, s lower.Assign, value Value) error {
	return assignQualifiedForeign(ec, s, value, h.describe())
}

// assignQualifiedForeign is the action and state share of assignForeign: the
// qualifier names the run or object owning the feature, so the write lands on
// the frame a run of it binds, else on the performing object it types.
func assignQualifiedForeign(ec *EvalContext, s lower.Assign, value Value, describe string) error {
	if written, err := ec.writeFrameFeature(s.Owner, s.Feature, value); err != nil {
		return err
	} else if written {
		return nil
	}
	target := ec.self
	if target != nil && ec.ctx.isOrSpecializes(target.Type, s.Owner) {
		if _, ok := target.FeatureValues[s.Target]; !ok {
			return fmt.Errorf("%s: object #%d (%s) has no feature %s",
				ErrNoSuchFeature, target.ID, SymbolText(target.Type), s.Target)
		}
		if err := target.SetFeatureValue(ec.ctx, s.Target, value); err != nil {
			return err
		}
		ec.ctx.noteObjectWrite(target, s.Target, value)
		return nil
	}
	return fmt.Errorf("%s: assignment to %s::%s names no object typed by %s to write on",
		describe, s.Owner.Name, s.Target, s.Owner.Name)
}

// performer is the object performing the action this body belongs to.
func (h *actionStmtHost) performer() *Instance {
	return h.exec.self
}

// occurrence is the performance instance this action runs as: `this` in a body
// statement denotes it.
func (h *actionStmtHost) occurrence() *Instance {
	return h.exec.occurrence
}

// materializeOccurrence defers to the performance's lazy hook, set on the
// executor that runs it.
func (h *actionStmtHost) materializeOccurrence() (*Instance, error) {
	if h.exec.thisOccurrence == nil {
		return nil, nil
	}
	return h.exec.thisOccurrence()
}

// acceptReturn rejects a `return`: an action node computes no result to return.
// statementOrder is how stmts may be ordered where the schedule picks it: a body's
// statements are subactions no succession orders.
func (h *actionStmtHost) statementOrder(stmts []lower.Statement) *lower.StatementOrder {
	if len(stmts) < 2 {
		return nil
	}
	order := h.exec.statementOrder(h.graph, nil, h.node, stmts)
	if h.exec.ctx.scheduling().ordersStatements() || order.HasReversePrecedence() || order.HasSkipped() {
		return order
	}
	return nil
}

func (h *actionStmtHost) statementFootprint(stmts []lower.Statement, i int) lower.Footprint {
	order := h.exec.statementOrder(h.graph, nil, h.node, stmts)
	fp := order.Footprint(i)
	if h.perf == nil || h.perf.node == nil {
		return fp
	}
	// The stamped footprint depends only on the statement and the performed
	// node, so each performance of a node computes it once, not per execution.
	list, keyed := statementListKey(h.node, stmts)
	key := stampedFootprint{list: list, i: i, node: h.perf.node}
	model := h.exec.ctx.model
	if stamped, done := model.stamped[key]; done && keyed {
		return stamped
	}
	// The node's own features are this performance's places, as its pins are;
	// another node's pin the body reads stays that node's.
	own := make(map[ast.Node]bool)
	for _, f := range h.graph.Features[h.perf.node] {
		own[f.Node] = true
	}
	fp.Reads, fp.Writes = placesOf(fp.Reads, h.perf.node, own), placesOf(fp.Writes, h.perf.node, own)
	if !keyed {
		return fp
	}
	if model.stamped == nil {
		model.stamped = make(map[stampedFootprint]lower.Footprint)
	}
	model.stamped[key] = fp
	return fp
}

// placesOf is the places with those declared by node's own features, the
// declarations in own, stamped as node's; the places themselves when none is.
func placesOf(places []lower.Place, node ast.Node, own map[ast.Node]bool) []lower.Place {
	stamps := func(p lower.Place) bool { return p.Node == nil && p.Sym != nil && own[p.Sym.Decl] }
	if !slices.ContainsFunc(places, stamps) {
		return places
	}
	out := slices.Clone(places)
	for i := range out {
		if stamps(out[i]) {
			out[i].Node = node
		}
	}
	return out
}

func (h *actionStmtHost) orderStep() int {
	if h.exec.flow == nil {
		return 0
	}
	return h.exec.flow.stepCount + 1
}

func (h *actionStmtHost) yieldsBetweenStatements() bool { return true }

func (h *actionStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s", errReturnOutsideCalc, h.describe())
}

// effect performs the action a `perform` in statement form names, where it
// stands, starts the behavior a `perform obj.beh.start` names on its object, or
// ends the performance a `terminate` names; any other effect is reported.
func (h *actionStmtHost) effect(engine *stmtEngine, s lower.Effect) error {
	env := engine.env
	if s.Kind == lower.EffectTerminate {
		return h.exec.terminate(engine, h.around(), s)
	}
	if s.Kind == lower.EffectStart {
		if err := h.exec.ctx.startEffect(engine.evalIn(s.Scope), s, h.exec.self); err != nil {
			return fmt.Errorf("%s: %w", h.describe(), err)
		}
		return nil
	}
	if s.Kind != lower.EffectPerform {
		return fmt.Errorf("%s: '%s' in a body is not executable", h.describe(), s.Kind)
	}
	inv, ok := performedInvocation(s)
	if !ok {
		return fmt.Errorf("%s: 'perform' names no action to perform", h.describe())
	}
	// The performed action reads the values in scope where it is performed and its
	// outputs come back to them, so a perform in a loop body sees that iteration.
	values, err := env.values(h.exec.ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", h.describe(), err)
	}
	_, outputs, err := invokeAction(h.exec.ctx, s.Scope, inv, values, h.exec.self)
	if err != nil {
		return fmt.Errorf("%s: %w", h.describe(), err)
	}
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if env.assign(h.exec.ctx, name, outputs[name]) {
			continue
		}
		if written, err := h.exec.returnEnclosing(h.perf, name, outputs[name], h.exec.owner.assignAround); written || err != nil {
			if err != nil {
				return fmt.Errorf("%s: %w", h.describe(), err)
			}
			continue
		}
		env.data.setBody(h.exec.ctx, name, outputs[name])
	}
	return nil
}

// performNode performs a nested action a block of the body declares as a
// subperformance of the body's.
func (h *actionStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	return h.exec.performNode(h.around(), engine, graph, node)
}

// runFlow rejects a stated flow among statements: an action's own flow is the
// executor's, and a block of its body sequences its nodes.
func (h *actionStmtHost) runFlow(lower.Block) (stmtFlow, error) {
	return flowNext, fmt.Errorf("%w: %s: a flow of steps in a block is not executable",
		errStatementNotExecutable, h.describe())
}

func (h *actionStmtHost) runBlockFlow(engine *stmtEngine, block lower.Block) (stmtFlow, error) {
	return h.exec.performBlockFlow(h.around(), engine, block)
}

// performNode performs node, which a block of parent's body declares, as a subperformance
// of parent with the block-locals entered around it in reach; a node owning a flow runs it
// to completion here. A breakpoint on the node pauses the run before it performs.
func (e *performances) performNode(parent *actionFrame, engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	f, resumed, err := popFrame[*performFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &performFrame{levels: e.ctx.bodyLevels()}
	}
	if !resumed || f.recheck {
		f.recheck = false
		if err := e.owner.pauseAt(parent.within(), node); err != nil {
			return flowNext, e.ctx.pausing(f, err)
		}
	}
	if f.perf == nil {
		locals, localCells, localHeld := engine.env.localFrames()
		if f.perf, err = e.beginPerformance(
			parent, graph, node, locals, localCells, localHeld,
		); err != nil {
			return flowNext, err
		}
		if _, declared := graph.Multiplicities[node]; declared && f.perf.repeatedStep() {
			e.recordRepetition(parent, node, f.perf)
		}
	}
	// A terminate of the node ends its body where it stands, dropping what a flow nested in
	// its leaf body still runs (runSubflow drops a flow of its own); the node completes.
	// One ended while the body was paused (endPerformed) has only the node to complete.
	var ended *terminated
	if !f.ended {
		if err := e.performNodeBody(f, graph, node); err != nil {
			err = e.terminatedUsage(f.perf, graph, err)
			if ended = unwound(err); ended == nil {
				return flowNext, e.ctx.pausing(f, err)
			}
			if ended.perf != f.perf {
				// The node ends a performance around it: its own completes first, pins bound.
				if err := e.endPerformance(f.perf); err != nil {
					return flowNext, err
				}
				return flowNext, ended
			}
			if f.perf.graph == nil {
				e.flow.dropTokensIn(f.perf, 0)
			}
		}
		if err := e.endPerformance(f.perf); err != nil {
			return flowNext, err
		}
	}
	if err := e.applyDataFlows(parent, graph, node, f.perf, f.perf.data, f.perf.streamed, nil); err != nil {
		return flowNext, err
	}
	if ended != nil {
		return flowNext, e.flow.endAlongside(ended)
	}
	return flowNext, nil
}

// markPerformance records what performing node binds, for the marks of the run's
// values: as it begins, its own features and pins stand on what they are bound
// from; once ended, its outputs — the pins and the enclosing features they are
// carried to — stand on what its body left on its pins. The body's statements
// are recorded one by one as they run, so an ended performance's pins are not
// bound again from what its initializers read.
func (e *performances) markPerformance(graph *lower.ActionGraph, node ast.Node, ended bool) {
	fp := graph.BindingFootprints()[node]
	if !ended {
		own := slices.DeleteFunc(slices.Clone(fp.Writes), func(w lower.Place) bool { return !w.Local })
		e.ctx.writeMarks(own, e.ctx.standingOf(fp, false))
		return
	}
	by := slices.Clone(e.ctx.controls)
	for _, w := range fp.Writes {
		if w.Local {
			by = unionSerials(by, e.ctx.run.marks.at(w))
		}
	}
	e.ctx.writeMarks(fp.Writes, by)
}

// performFrame is a node a body performs (performNode) where the body paused: at
// a breakpoint before the node's performance began, else in one of its phases.
type performFrame struct {
	perf  *actionFrame
	phase performPhase
	// recheck has the resumed body look for a breakpoint on the node again: the
	// one it stopped at was removed while it stood, so one set since is a new stop.
	recheck bool
	// levels is the trace nesting the body held open at the node, over the depth
	// its run resumed at; ended marks the performance a terminate ended meanwhile.
	levels int
	ended  bool
}

// stoppedAtBreakpoint reports whether the frame is a body's stop at a breakpoint,
// before the node performs.
func (f *performFrame) stoppedAtBreakpoint() bool { return f.perf == nil }

// abandon ends the node's performance with the body that was performing it.
func (f *performFrame) abandon(*Context) {
	if f.perf != nil {
		f.perf.ended, f.perf.live = true, 0
	}
}

func (f *performFrame) clone() bodyFrame { c := *f; return &c }

// blockFlowFrame resumes a transparent performance for a stated body flow.
type blockFlowFrame struct {
	perf   *actionFrame
	levels int
}

func (f *blockFlowFrame) abandon(*Context) {
	if f.perf != nil {
		f.perf.ended, f.perf.live = true, 0
	}
}

func (f *blockFlowFrame) clone() bodyFrame { c := *f; return &c }

// performBlockFlow runs the flow a loop or branch body states as a performance
// of its own, a subperformance of parent with engine's block-locals in reach.
func (e *performances) performBlockFlow(parent *actionFrame, engine *stmtEngine, block lower.Block) (stmtFlow, error) {
	f, resumed, err := popFrame[*blockFlowFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &blockFlowFrame{levels: e.ctx.bodyLevels()}
	}
	if f.perf == nil {
		scope := block.Scope
		if scope == nil {
			scope = parent.scope
		}
		locals, localCells, localHeld := engine.env.localFrames()
		f.perf = &actionFrame{
			node:        block.Node,
			graph:       block.Graph,
			flow:        block.Graph,
			scope:       scope,
			parent:      parent,
			locals:      locals,
			localCells:  localCells,
			localHeld:   localHeld,
			connections: joinConnections(parent.connections, block.Graph.Connections),
			data:        make(map[string]Value),
			features:    make(map[string]ast.FeatureDirection),
			subactions:  make(map[ast.Node]*actionFrame),
			perfs:       e,
			run:         e.ctx.newRun(),
			live:        1,
			body:        true,
		}
		switch block.Node.(type) {
		case *ast.WhileLoopActionNode:
			f.perf.label = "loop body of " + parent.describe()
		case *ast.IfBranchNode:
			f.perf.label = "branch body of " + parent.describe()
		}
		features := make([]lower.Feature, 0, len(block.Graph.Attributes))
		for _, attr := range block.Graph.Attributes {
			f.perf.features[attr.Name] = attr.Direction
			features = append(features, lower.Feature{
				Name: attr.Name, Direction: attr.Direction, IsResult: attr.IsResult, Binding: attr.Binding,
				Value: attr.Value, Node: attr.Node, Scope: block.Scope,
			})
		}
		activation, endStep := e.ctx.beginStep()
		f.perf.began = activation
		if err := e.seedDeclaredValues(f.perf, features, activation); err != nil {
			endStep()
			return flowNext, err
		}
		endStep()
	}
	err = e.owner.runOwnFlow(f.perf)
	if err != nil {
		if ended := unwound(err); ended != nil && ended.perf != f.perf {
			e.flow.dropTokensIn(f.perf, 0)
			f.perf.live = 0
			f.perf.ended = true
			return flowNext, ended
		}
		if paused(err) {
			return flowNext, e.ctx.pausing(f, err)
		}
		return flowNext, err
	}
	f.perf.ended = true
	return flowNext, nil
}

// performPhase is how far a node's performance has come.
type performPhase int

const (
	performInvoking performPhase = iota // the case it is or the action it performs
	performBody                         // the flow it owns or its statements
)

// performNodeBody performs the case the node is or the action it performs, then
// the flow it owns or the statements of its body.
func (e *performances) performNodeBody(f *performFrame, graph *lower.ActionGraph, node *ast.Usage) error {
	perf := f.perf
	if isCaseStep(node) {
		return e.performCase(perf)
	}
	if f.phase == performInvoking {
		if inv, ok := nestedInvocationInGraph(graph, node); ok {
			if err := e.performInvocation(perf, inv); err != nil {
				return err
			}
		}
		f.phase = performBody
	}
	if perf.graph != nil {
		return e.owner.runOwnFlow(perf)
	}
	return e.executeBody(perf, graph, node)
}

// declaredOutput reports no output features: an action node's parameters live
// in the action's feature space, which an assignment writes directly.
func (h *actionStmtHost) declaredOutput(string) bool {
	return false
}
