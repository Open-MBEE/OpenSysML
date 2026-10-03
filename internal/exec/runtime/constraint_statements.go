package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// constraintStmtHost runs the steps of one constraint body's Boolean
// performance: it owns its locals and a copy of the constraint's parameters,
// and refuses every effect reaching outside that performance — a write of the
// constrained object's features, a send, a performed action, a terminate —
// and the stated flow a verdict orders by declaration instead.
type constraintStmtHost struct {
	ctx   *Context
	self  *Instance
	scope *symbols.Scope
}

// describe names the host in a diagnostic.
func (h *constraintStmtHost) describe() string { return "constraint body" }

// send refuses the message: sending it is an effect outside the performance.
func (h *constraintStmtHost) send(*EvalContext, lower.Send) error {
	return fmt.Errorf("%w: %s: a send addresses the world the constraint judges", ErrConstraintEffect, h.describe())
}

// declaredOutput reports false: a constraint performance declares no output.
func (h *constraintStmtHost) declaredOutput(string) bool { return false }

// assignOuter writes a parameter of the constraint into the performance's own
// copy of it (SysML v2 §7.17.9: the implicit target of an assignment is the
// constraint's own performance), and refuses any other name — that names a
// feature of the constrained object or an enclosing frame, not of the
// performance, and writing it is an effect no verdict performs.
func (h *constraintStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if h.isParameter(name) {
		return storeBodyValue(h.ctx, h, env, name, value, s)
	}
	return fmt.Errorf("%w: %s: the implicit target of an assignment is the constraint's own performance (SysML v2 §7.17.9) and %s is not one of its features",
		ErrConstraintExternalAssignment, h.describe(), name)
}

// isParameter reports whether name declares a parameter the body's own scope
// holds (`in x`) — the features the constraint's performance owns.
func (h *constraintStmtHost) isParameter(name string) bool {
	if h.scope == nil {
		return false
	}
	sym, ok := h.scope.LookupLocal(name)
	if !ok || sym == nil {
		return false
	}
	u, ok := sym.Decl.(*ast.Usage)
	return ok && (u.Direction == ast.DirIn || u.Direction == ast.DirInOut)
}

// assignData writes a name the performance already holds — a local or a
// parameter copy — checked against its declaration.
func (h *constraintStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	return storeBodyValue(h.ctx, h, env, name, value, s)
}

// assignChain refuses a chained target: it writes a feature of the object the
// chain reaches, an effect outside the performance.
func (h *constraintStmtHost) assignChain(_ *EvalContext, s lower.Assign, _ Value) error {
	return fmt.Errorf("%w: %s: %s writes a feature of an object outside the constraint's own performance",
		ErrConstraintEffect, h.describe(), s.Chain.Text)
}

// assignForeign refuses a qualified target for the same reason a chained one is.
func (h *constraintStmtHost) assignForeign(_ *EvalContext, s lower.Assign, _ Value) error {
	return fmt.Errorf("%w: %s: %s::%s writes a feature of an object outside the constraint's own performance",
		ErrConstraintEffect, h.describe(), s.Owner.Name, s.Target)
}

// acceptReturn refuses a `return`: a verdict is no caller a value returns to.
func (h *constraintStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s: `return` answers a caller, which a constraint body's verdict has none of",
		ErrStatementNotExecutable, h.describe())
}

// effect refuses what performs on the world outside the performance — an
// action performed, an accept, a terminate — which a verdict does not perform.
func (h *constraintStmtHost) effect(_ *stmtEngine, s lower.Effect) error {
	return fmt.Errorf("%w: %s: the `%s` statement acts outside the constraint's own performance",
		ErrConstraintEffect, h.describe(), s.Kind)
}

// performNode runs a nested action of a block in a frame of the body's, as a
// calculation body's host does; one performing an action of its own acts
// outside the performance, and one stating a flow or a pin connection is the
// stated flow a verdict does not order.
func (h *constraintStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	if _, performs := nestedInvocation(node); performs {
		return flowNext, fmt.Errorf("%w: %s: performing action %s is an effect outside the constraint's own performance",
			ErrConstraintEffect, h.describe(), ActionNodeName(node))
	}
	if connectsPins(graph, node) {
		return flowNext, fmt.Errorf("%s: a binding or flow at a pin of %s in a body is not executable",
			h.describe(), nodeDescription(node))
	}
	if sub, owns := graph.Subflows[node]; owns && sub != nil {
		return flowNext, fmt.Errorf("%w: %s: the flow %s states of its own in a body is not executable",
			ErrStatementNotExecutable, h.describe(), nodeDescription(node))
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
	return engine.run(graph.Bodies[node])
}

// runBlockFlow refuses the flow a loop or branch body states: a verdict orders
// its steps by declaration, not by the successions stated.
func (h *constraintStmtHost) runBlockFlow(engine *stmtEngine, block lower.Block) (stmtFlow, error) {
	return flowNext, fmt.Errorf("%w: %s: the flow a body states in a constraint is not executable",
		ErrStatementNotExecutable, h.describe())
}

// runFlow refuses the stated flow of a constraint body for the same reason.
func (h *constraintStmtHost) runFlow(block lower.Block) (stmtFlow, error) {
	return flowNext, fmt.Errorf("%w: %s: a flow of steps in a body is not executable",
		ErrStatementNotExecutable, h.describe())
}

// performer is the object the constraint is checked on: what the body's names
// read through, and never write — an assignment's implicit target is the
// performance itself.
func (h *constraintStmtHost) performer() *Instance { return h.self }

// occurrence is nil: a constraint's verdict materializes no occurrence for `this`.
func (h *constraintStmtHost) occurrence() *Instance { return nil }

// materializeOccurrence is nil for the same reason occurrence is.
func (h *constraintStmtHost) materializeOccurrence() (*Instance, error) { return nil, nil }

// runConstraintSteps runs the body's steps as one performance in a fresh frame:
// the check's frames and bindings enclose it, the constraint's parameters are
// copied into it as the steps write them, and nothing is written back — into
// the constrained object, the caller's bindings, or an enclosing frame. It
// answers the frame the steps left, which the body's conditions read innermost.
func (ctx *Context) runConstraintSteps(steps *BodySteps, features map[string]scopedExpr, self *Instance, frames []frame, bindings frame) (frame, error) {
	data := frame{vars: make(map[string]Value), unvalued: make(map[string]bool)}
	enclosing := make([]frame, 0, len(frames)+1)
	enclosing = append(enclosing, frames...)
	if bindings.vars != nil {
		enclosing = append(enclosing, bindings)
	}
	host := &constraintStmtHost{ctx: ctx, self: self, scope: steps.Scope}
	_, err := ctx.runStatements(func() *stmtEngine {
		engine := newStmtEngineIn(ctx, host, data, enclosing)
		engine.features = features
		return engine
	}, steps.Stmts)
	if err != nil {
		return frame{}, err
	}
	return data, nil
}
