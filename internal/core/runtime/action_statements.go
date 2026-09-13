package runtime

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// actionStmtHost runs an action node's body statements: a send posts through the flow's
// connections, and an assignment to an undeclared name reaches the enclosing performances.
type actionStmtHost struct {
	exec *performances
	node ast.Node // the action node whose body is running, for diagnostics
	// perf is the performance the body runs in, whose features it declares into.
	perf *actionFrame
	// engine runs the body, and holds the values a `perform` in it reads and
	// writes: the performance's own, and those of every block entered around it.
	engine *stmtEngine
}

// executeBody runs the lowered statements graph records for node in perf, the
// performance they belong to, with the performances around it in lexical reach.
func (e *performances) executeBody(perf *actionFrame, graph *lower.ActionGraph, node ast.Node) error {
	host := &actionStmtHost{exec: e, node: node, perf: perf}
	lexical := perf.lexicalFrames()
	engine := newStmtEngineIn(e.ctx, host, lexical[len(lexical)-1], lexical[:len(lexical)-1])
	host.engine = engine
	// The body's activation ends with this execution of it, so a run stepping the
	// node many times does not hold what every execution computed.
	defer engine.finish()
	_, err := engine.run(graph.Bodies[node])
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
	return h.exec.ctx.send(ec, h.exec.root.scope, h.perf.connections, s, h.exec.self)
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
// stands; any other effect is reported.
func (h *actionStmtHost) effect(s lower.Effect) error {
	if s.Kind != lower.EffectPerform {
		return fmt.Errorf("%s: '%s' in a body is not executable", h.describe(), s.Kind)
	}
	inv, ok := performedInvocation(s)
	if !ok {
		return fmt.Errorf("%s: 'perform' names no action to perform", h.describe())
	}
	// The performed action reads the values in scope where it is performed and its
	// outputs come back to them, so a perform in a loop body sees that iteration.
	env := h.engine.env
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
	if err := e.owner.pauseAt(node); err != nil {
		return flowNext, err
	}
	perf, err := e.beginPerformance(parent, graph, node, slices.Clone(engine.env.frames))
	if err != nil {
		return flowNext, err
	}
	if isCaseStep(node) {
		if err := e.performCase(perf); err != nil {
			return flowNext, err
		}
	} else {
		if inv, ok := nestedInvocation(node); ok {
			if err := e.performInvocation(perf, inv); err != nil {
				return flowNext, err
			}
		}
		if perf.graph != nil {
			if err := e.owner.runOwnFlow(perf); err != nil {
				return flowNext, err
			}
		} else if err := e.executeBody(perf, graph, node); err != nil {
			return flowNext, err
		}
	}
	if err := e.endPerformance(perf); err != nil {
		return flowNext, err
	}
	return flowNext, e.applyDataFlows(parent, graph, node, perf.data)
}

// declaredOutput reports no output features: an action node's parameters live
// in the action's feature space, which an assignment writes directly.
func (h *actionStmtHost) declaredOutput(string) bool {
	return false
}
