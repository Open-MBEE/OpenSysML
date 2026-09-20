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
}

// executeBody runs the lowered statements graph records for node in perf, the
// performance they belong to, with the performances around it in lexical reach.
func (e *performances) executeBody(perf *actionFrame, graph *lower.ActionGraph, node ast.Node) error {
	_, err := e.ctx.runStatements(func() *stmtEngine {
		host := &actionStmtHost{exec: e, node: node, perf: perf}
		lexical := perf.lexicalFrames()
		return newStmtEngineIn(e.ctx, host, lexical[len(lexical)-1], lexical[:len(lexical)-1])
	}, graph.Bodies[node])
	return err
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

// performer is the object performing the action this body belongs to.
func (h *actionStmtHost) performer() *Instance {
	return h.exec.self
}

// acceptReturn rejects a `return`: an action node computes no result to return.
func (h *actionStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s", ErrReturnOutsideCalc, h.describe())
}

// effect performs the action a `perform` in statement form names, where it
// stands, or ends the performance a `terminate` names; any other effect is reported.
func (h *actionStmtHost) effect(engine *stmtEngine, s lower.Effect) error {
	env := engine.env
	if s.Kind == lower.EffectTerminate {
		return h.exec.terminate(engine, h.perf, s)
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
	_, outputs, err := invokeAction(h.exec.ctx, s.Scope, inv, env.values(), h.exec.self)
	if err != nil {
		return fmt.Errorf("%s: %w", h.describe(), err)
	}
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if env.assign(name, outputs[name]) {
			continue
		}
		if written, err := h.exec.assignEnclosing(h.perf, name, outputs[name]); written || err != nil {
			if err != nil {
				return fmt.Errorf("%s: %w", h.describe(), err)
			}
			continue
		}
		env.data.set(name, outputs[name])
	}
	return nil
}

// performNode performs a nested action a block of the body declares as a
// subperformance of the body's.
func (h *actionStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	return h.exec.performNode(h.perf, engine, graph, node)
}

// runFlow rejects a stated flow among statements: an action's own flow is the
// executor's, and a block of its body sequences its nodes.
func (h *actionStmtHost) runFlow(lower.Block) (stmtFlow, error) {
	return flowNext, fmt.Errorf("%w: %s: a flow of steps in a block is not executable",
		ErrStatementNotExecutable, h.describe())
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
		if f.perf, err = e.beginPerformance(parent, graph, node, slices.Clone(engine.env.frames)); err != nil {
			return flowNext, err
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
	if err := e.applyDataFlows(parent, graph, node, f.perf.data, f.perf.streamed); err != nil {
		return flowNext, err
	}
	if ended != nil {
		return flowNext, e.flow.endAlongside(ended)
	}
	return flowNext, nil
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
		if inv, ok := nestedInvocation(node); ok {
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
