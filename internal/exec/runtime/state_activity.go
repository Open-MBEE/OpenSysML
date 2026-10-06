package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// stateActivity answers a chain ending in StateActivity::isActive from the
// configuration of the machine holding the state (StateExecutor.Activity): the
// enclosing machine, else one the object read on or the operand's object exhibits.
func (ec *EvalContext) stateActivity(n *ast.FeatureChainExpr, base ast.Node, parts []ast.NameSegment) (Value, bool, error) {
	if ec.ctx.model.semantics == nil || ec.ctx.model.resolver == nil || len(parts) == 0 {
		return Value{}, false, nil
	}
	sym, ok := ec.ctx.resolveTarget(ec.scope, n)
	if !ok || !semantics.IsStateActivity(sym) {
		return Value{}, false, nil
	}
	operand := ec.ctx.model.activityOperand(n)
	if machine, ok := ec.enclosingMachine(); ok {
		if val, ok := ec.activityOf(machine, operand); ok {
			return val, true, nil
		}
	}
	for _, inst := range ec.activityObjects(base, parts[:len(parts)-1]) {
		for _, b := range inst.ExhibitedStates() {
			if b.State == nil {
				continue
			}
			if val, ok := ec.activityOf(b.State, operand); ok {
				return val, true, nil
			}
		}
	}
	return boolValue(false), true, nil
}

// activityOf is the activity of the state operand names within machine, or of
// the machine itself when operand names the usage it runs as; false when
// machine holds neither.
func (ec *EvalContext) activityOf(machine *StateExecutor, operand ast.Node) (Value, bool) {
	if state, ok := ec.activityState(machine, operand); ok {
		return boolValue(machine.Activity(state)), true
	}
	if usage, ok := ec.ctx.resolveTarget(ec.scope, operand); ok && machine.performs(usage) {
		return boolValue(machine.State() == StateRunning || machine.State() == StateWaiting), true
	}
	return Value{}, false
}

// activityState is the state of machine the operand of a read names: a vertex
// path in the scope the read was written in (`outer.inner` within the machine),
// else a path through the usage the machine runs as (`modes.ready` from the part
// exhibiting it), whose remainder names the vertex within the machine.
func (ec *EvalContext) activityState(machine *StateExecutor, operand ast.Node) (*ast.StateNode, bool) {
	if state, ok := machine.graph.StateNamed(ec.scope, operand); ok {
		return state, true
	}
	chain, ok := operand.(*ast.FeatureChainExpr)
	if !ok || chain.Member == nil {
		return nil, false
	}
	for i := range chain.Member.Parts {
		split := ec.ctx.model.activitySplitAt(chain, i)
		usage, ok := ec.ctx.resolveTarget(ec.scope, split.machine)
		if !ok || !machine.performs(usage) {
			continue
		}
		if state, ok := machine.graph.StateNamed(nil, split.state); ok {
			return state, true
		}
	}
	return nil, false
}

// activitySplitKey keys an operand chain split before its member i.
type activitySplitKey struct {
	chain *ast.FeatureChainExpr
	at    int
}

// activitySplit is an operand chain split into the node naming a machine usage
// and the node naming a state path within that machine.
type activitySplit struct {
	machine ast.Node
	state   ast.Node
}

// activitySplitAt splits the operand chain `m.a.b` before its member at: `m`
// and `a.b` for 0, `m.a` and `b` for 1; memoized so the resolver sees one node
// per split.
func (m *Model) activitySplitAt(chain *ast.FeatureChainExpr, at int) activitySplit {
	key := activitySplitKey{chain: chain, at: at}
	if split, ok := m.activitySplits[key]; ok {
		return split
	}
	parts := chain.Member.Parts
	split := activitySplit{machine: chain.Operand, state: featurePathNode(parts[at:])}
	if at > 0 {
		split.machine = &ast.FeatureChainExpr{
			NodeBase: ast.NodeBase{NodeSpan: chain.NodeSpan},
			Operand:  chain.Operand,
			Member:   &ast.QualifiedName{NodeBase: ast.NodeBase{NodeSpan: parts[0].Span}, Parts: parts[:at]},
		}
	}
	m.activitySplits[key] = split
	return split
}

// featurePathNode is the node reading the feature path parts spells: a feature
// reference for one name, else a chain from the first name through the rest.
func featurePathNode(parts []ast.NameSegment) ast.Node {
	base := &ast.NodeBase{NodeSpan: parts[0].Span}
	head := &ast.FeatureReference{
		NodeBase: *base,
		Name:     &ast.QualifiedName{NodeBase: *base, Parts: parts[:1]},
	}
	if len(parts) == 1 {
		return head
	}
	return &ast.FeatureChainExpr{
		NodeBase: *base,
		Operand:  head,
		Member:   &ast.QualifiedName{NodeBase: ast.NodeBase{NodeSpan: parts[1].Span}, Parts: parts[1:]},
	}
}

// activityOperand is the node naming the state a read of its activity is of:
// the chain's operand when `isActive` is the only member, else the chain up to
// the member before it, memoized so the resolver sees one node per read.
func (m *Model) activityOperand(n *ast.FeatureChainExpr) ast.Node {
	if len(n.Member.Parts) == 1 {
		return n.Operand
	}
	if operand, ok := m.activityOperands[n]; ok {
		return operand
	}
	parts := n.Member.Parts[:len(n.Member.Parts)-1]
	operand := &ast.FeatureChainExpr{
		NodeBase: ast.NodeBase{NodeSpan: n.NodeSpan},
		Operand:  n.Operand,
		Member:   &ast.QualifiedName{NodeBase: ast.NodeBase{NodeSpan: parts[0].Span}, Parts: parts},
	}
	m.activityOperands[n] = operand
	return operand
}

// enclosingMachine is the state machine whose data frame the context reads
// within, directly or around the action performance it evaluates in.
func (ec *EvalContext) enclosingMachine() (*StateExecutor, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if machine := ec.frames[i].machine; machine != nil {
			return machine, true
		}
		if perf := ec.frames[i].perf; perf != nil {
			for root := perf; root != nil; root = root.parent {
				for j := len(root.outer) - 1; j >= 0; j-- {
					if machine := root.outer[j].machine; machine != nil {
						return machine, true
					}
				}
			}
		}
	}
	return nil, false
}

// activityObjects are the objects whose exhibited machines may hold the state a
// read of its activity names: the object the read's feature names resolve
// against, the occurrence the read is made on and the object the chain's
// operand, read up to the machine usage, denotes.
func (ec *EvalContext) activityObjects(base ast.Node, statePath []ast.NameSegment) []*Instance {
	var objects []*Instance
	for _, inst := range []*Instance{ec.self, ec.occurrence} {
		if inst != nil && !slices.Contains(objects, inst) {
			objects = append(objects, inst)
		}
	}
	sym, named := ec.chainBaseSymbol(base)
	if named && sym != nil {
		if inst := ec.ctx.denotedInstance(sym); inst != nil && !slices.Contains(objects, inst) {
			objects = append(objects, inst)
		}
	}
	if named && isStateSymbol(sym) {
		// The chain opens on a state, so it is a state path, not an object path.
		return objects
	}
	for i := len(statePath) - 1; i >= 0; i-- {
		val, err := ec.evalChainPrefix(base, statePath[:i])
		if err != nil || val.Kind != ValInstance {
			continue
		}
		if inst, ok := ec.ctx.instances[val.Instance]; ok && !slices.Contains(objects, inst) {
			objects = append(objects, inst)
		}
	}
	return objects
}

// evalChainPrefix evaluates base followed by parts, as the chain they open reads.
func (ec *EvalContext) evalChainPrefix(base ast.Node, parts []ast.NameSegment) (Value, error) {
	val, err := ec.Eval(base)
	if err != nil || len(parts) == 0 {
		return val, err
	}
	return ec.chainMemberValue(val, parts, chainText(base))
}

// denotedInstance is the object sym denotes when it is a part already
// materialized, nil otherwise.
func (ctx *Context) denotedInstance(sym *symbols.Symbol) *Instance {
	val, err := ctx.denotedValue(sym)
	if err != nil || val.Kind != ValInstance {
		return nil
	}
	return ctx.instances[val.Instance]
}

// performs reports whether sym names the machine itself: the usage or
// definition it runs, or one bound to it (`exhibit state m : M`).
func (e *StateExecutor) performs(sym *symbols.Symbol) bool {
	if e == nil || sym == nil || sym.Decl == nil {
		return false
	}
	if e.stateMachine != nil && e.stateMachine.Decl == sym.Decl {
		return true
	}
	if e.self == nil {
		return false
	}
	for _, b := range e.self.ExhibitedStatesOf(sym) {
		if b.State == e {
			return true
		}
	}
	return false
}
