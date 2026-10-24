package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A loop an action body reaches is a LoopAction performance of its own, held under the
// performing action's `loops` (Actions.sysml `loops : LoopAction[0..*] :> subactions`),
// and each iteration performs the loop's body as an occurrence held under the loop's
// `body` (`LoopAction::body`; ControlPerformances.kerml `LoopPerformance::body`).
const (
	loopsFQN           = "Actions::Action::loops"
	loopBodyFQN        = "Actions::LoopAction::body"
	whileLoopActionFQN = "Actions::WhileLoopAction"
	forLoopActionFQN   = "Actions::ForLoopAction"
)

// loopPerformer is a statement host whose loops are performances: an action body's,
// where the loop and each iteration of its body are objects of the run.
type loopPerformer interface {
	beginLoop(stmt lower.Loop) (*Instance, error)
}

// beginLoopPerformance begins the loop statement stmt reaches in frame as a
// performance held under the object of the performance around it; nil where that
// holds no object or the library declares no loop action, and the loop runs as before.
func (e *performances) beginLoopPerformance(frame *actionFrame, stmt lower.Loop) (*Instance, error) {
	// A loop node of a flow performs nothing but its loop: the frame is the loop's.
	_, loopNode := frame.node.(*ast.WhileLoopActionNode)
	around := frame
	if loopNode {
		around = frame.parent
	}
	holder, err := e.frameObject(around)
	if holder == nil || err != nil {
		return nil, err
	}
	fqn := whileLoopActionFQN
	if stmt.Kind == ast.LoopFor {
		fqn = forLoopActionFQN
	}
	typ := e.ctx.librarySymbol(fqn)
	if typ == nil {
		return nil, nil
	}
	inst, err := e.ctx.holdKernelMember(holder, loopsFQN, typ)
	if inst == nil || err != nil {
		return nil, err
	}
	// The body is held from the start: `[]` until the first iteration, never refused.
	if _, err := e.ctx.kernelMemberHolding(inst, loopBodyFQN); err != nil {
		return nil, err
	}
	e.ctx.beginPerformanceLife(inst, e.ctx.newActivation())
	if loopNode {
		frame.occurrence = inst
	}
	return inst, nil
}

// frameObject is the object the performance frame runs as: the occurrence the run
// performs for the root (none for a definition run on its own), the object held for a
// loop or a loop body, for a named node the occurrence its name holds on the object
// around it, and for any other node — a control node, a branch — the object around it.
func (e *performances) frameObject(frame *actionFrame) (*Instance, error) {
	switch {
	case frame == nil:
		return nil, nil
	case frame.occurrence != nil:
		return frame.occurrence, nil
	case frame == e.root || frame.parent == nil:
		return e.occurrence, nil
	}
	owner, err := e.frameObject(frame.parent)
	if owner == nil || err != nil {
		return nil, err
	}
	usage, ok := frame.node.(*ast.Usage)
	if !ok || frame.body {
		return owner, nil
	}
	name := usage.Ident.Name
	if _, held := owner.FeatureValues[name]; name == "" || !held {
		return nil, nil
	}
	fv, err := owner.getFeatureValue(e.ctx, name, nil)
	if err != nil {
		return nil, err
	}
	elements, err := e.ctx.HeldElements(fv.HeldValue())
	if err != nil {
		return nil, err
	}
	at := max(frame.repetition-1, 0)
	if at >= int64(len(elements)) {
		return nil, nil
	}
	id, ok := elements[at].Object()
	if !ok {
		return nil, nil
	}
	inst, ok := e.ctx.Instance(id)
	if !ok {
		return nil, nil
	}
	return inst, nil
}

// kernelMemberHolding is the feature value owner holds the objects of the library
// member fqn under — a collection the object's type inherits (`loops`, `body`) —
// made empty when the object holds none yet; nil where the type has no such member.
func (ctx *Context) kernelMemberHolding(owner *Instance, fqn string) (*FeatureValue, error) {
	lib := ctx.librarySymbol(fqn)
	if lib == nil {
		return nil, nil
	}
	sym, found := ctx.model.semantics.LookupMember(owner.Type, lib.Name)
	if !found || sym == nil || !sym.IsFeature() {
		return nil, nil
	}
	name := lib.Name
	if fv, ok := owner.FeatureValues[name]; ok {
		return fv, nil
	}
	feat := ctx.effectiveFeature(name, sym, owner.Type)
	fv := &FeatureValue{Feature: &feat, Values: ctx.collectionOf(&feat, nil), Materialized: true}
	owner.FeatureValues[name] = fv
	ctx.noteProbeUndo(func() { delete(owner.FeatureValues, name) })
	return fv, nil
}

// holdKernelMember materializes an object of typ held by owner under the library member
// fqn names, appending it to what the member already holds; nil where the type has no
// such member.
func (ctx *Context) holdKernelMember(owner *Instance, fqn string, typ *symbols.Symbol) (*Instance, error) {
	fv, err := ctx.kernelMemberHolding(owner, fqn)
	if fv == nil || err != nil {
		return nil, err
	}
	name := fv.Feature.Name
	inst, err := ctx.materialize(typ, 0, owner, name)
	if err != nil {
		return nil, fmt.Errorf("hold %s of object #%d: %w", name, owner.ID, err)
	}
	return inst, ctx.appendHeld(fv, Value{Kind: ValInstance, Instance: inst.ID})
}

// appendHeld writes elem after what fv holds, as one write. A sequence grows into its
// spare storage, so a run appending one element at a time spends linear time and memory.
func (ctx *Context) appendHeld(fv *FeatureValue, elem Value) error {
	var held Value
	if fv.Materialized {
		held = fv.HeldValue()
	}
	var next Value
	if seq := held.Sequence(); seq != nil && seq.required == nil && !fv.Feature.HoldsSet {
		next = NewSequenceValue(seq.appended(elem))
	} else {
		elements, err := ctx.HeldElements(held)
		if err != nil {
			return err
		}
		next = ctx.collectionOf(fv.Feature, append(elements, elem))
	}
	ctx.noteProbeWrite(fv)
	endWrite := ctx.beginFeatureWrite(fv)
	before := ctx.beforeWrite(fv)
	fv.Value, fv.Values, fv.Materialized = Value{}, next, true
	ctx.afterWrite(fv, before)
	endWrite()
	return nil
}

// beginLoopPerformance begins the loop stmt as a performance of the host's, where
// the host performs its loops.
func (e *stmtEngine) beginLoopPerformance(stmt lower.Loop) (*Instance, error) {
	host, ok := e.host.(loopPerformer)
	if !ok {
		return nil, nil
	}
	return host.beginLoop(stmt)
}

// beginLoopBody begins one iteration of the loop's body as an occurrence held under
// the loop performance's `body`, typed by the body action the loop declares, else
// by the library's body parameter; it begins at activation, with the iteration.
func (e *stmtEngine) beginLoopBody(loop *Instance, stmt lower.Loop, activation int64) (*Instance, error) {
	typ := e.loopBodySymbol(stmt)
	if typ == nil {
		return nil, nil
	}
	inst, err := e.ctx.holdKernelMember(loop, loopBodyFQN, typ)
	if inst == nil || err != nil {
		return nil, err
	}
	e.ctx.beginPerformanceLife(inst, activation)
	return inst, nil
}

// loopBodySymbol is the action usage a loop declares as its body (`loop action body
// { … }`), else the library's `LoopAction::body` for a body declared without one.
func (e *stmtEngine) loopBodySymbol(stmt lower.Loop) *symbols.Symbol {
	if stmt.BodyAction != nil {
		return stmt.BodyAction
	}
	return e.ctx.librarySymbol(loopBodyFQN)
}

// loopBody is the object of the body iteration the engine's innermost loop is
// performing, nil where the loop runs without one.
func (e *stmtEngine) loopBody() *Instance {
	if n := len(e.loops); n > 0 {
		return e.loops[n-1].body
	}
	return nil
}
