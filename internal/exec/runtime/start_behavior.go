package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// startEffect runs a `perform obj.beh.start`: the object named (self for a bare
// `beh.start`) gets its own execution of beh, run to quiescence, and the statement is done.
func (ctx *Context) startEffect(ec *EvalContext, s lower.Effect, self *Instance) error {
	inst, member, err := ctx.startTarget(ec, s, self)
	if err != nil {
		return err
	}
	return ctx.startBehaviorOn(inst, member)
}

// startTarget is the object a start's chain names and the member of its type the
// behavior is bound by, resolved where the statement was written.
func (ctx *Context) startTarget(ec *EvalContext, s lower.Effect, self *Instance) (*Instance, *symbols.Symbol, error) {
	inst := self
	if chain, ok := s.Target.(*ast.FeatureChainExpr); ok {
		value, err := ec.Eval(chain.Operand)
		if err != nil {
			return nil, nil, fmt.Errorf("eval the object of %s: %w", exprText(s.TargetExpr), err)
		}
		if value.Kind != ValInstance {
			return nil, nil, fmt.Errorf("%w: %s is started on %s, which is no one object",
				ErrPerformerNotObject, exprText(s.Target), FormatValue(value))
		}
		if inst, ok = ctx.Instance(value.Instance); !ok {
			return nil, nil, fmt.Errorf("%w: %s is started on object #%d, which no longer exists",
				ErrPerformerNotObject, exprText(s.Target), value.Instance)
		}
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("%w: no object to start %s on", ErrNoSuchBehavior, exprText(s.Target))
	}
	member, ok := ctx.resolveReferenceTarget(s.Scope, s.Node, s.Target)
	if !ok || member == nil {
		return nil, nil, fmt.Errorf("unresolved behavior reference: %s", exprText(s.Target))
	}
	return inst, member, nil
}

// startBehaviorOn attaches the object's own execution of the behavior member binds and
// runs it to quiescence; an object already running it is left as it is. A failed start is undone whole.
func (ctx *Context) startBehaviorOn(inst *Instance, member *symbols.Symbol) error {
	decl, typ, ok := ctx.startableBehaviorOf(inst, member)
	if !ok {
		return fmt.Errorf("%w: %s is no behavior of object #%d (%s)",
			ErrNoSuchBehavior, symbolText(member), inst.ID, symbolText(inst.Type))
	}
	if err := ctx.checkPerformer(inst); err != nil {
		return fmt.Errorf("start %s %s: %w", decl.behavior.Kind, decl.behavior.Name, err)
	}
	if ctx.declarative || ctx.runsBound(inst, decl.member, typ) {
		return nil
	}
	defer ctx.beginRun()()
	defer ctx.holdDrivenWork()()
	commit, rollback := ctx.beginJournal()
	if ctx.trace != nil {
		ctx.trace.RecordBehaviorStart(decl.behavior.Kind.String(), decl.behavior.Name, inst.ID)
	}
	ctx.behaviorRunDepth++
	behavior, err := ctx.attachClassifierBehavior(inst, decl)
	ctx.behaviorRunDepth--
	if err != nil {
		rollback()
		return err
	}
	behavior.binding = ctx.bindingIndex(typ, decl.member)
	inst.behaviors = append(inst.behaviors, behavior)
	ctx.pendingBehaviors = append(ctx.pendingBehaviors, behavior)
	ctx.objectBehaviors = append(ctx.objectBehaviors, behavior)
	if err := ctx.runAttachedBehaviors(); err != nil {
		rollback()
		return err
	}
	commit()
	return nil
}

// startableBehaviorOf is the behavior member binds on the object and the type of it
// declaring or redefining it: one exhibited or performed, or a usage declared for a start.
func (ctx *Context) startableBehaviorOf(inst *Instance, member *symbols.Symbol) (classifierBehaviorDecl, *symbols.Symbol, bool) {
	for _, typ := range inst.types() {
		for _, decl := range ctx.classifierBehaviorsOf(typ) {
			if ctx.sameFeature(decl.member, member, typ) {
				return decl, typ, true
			}
		}
		for _, m := range ctx.model.semantics.MembersOf(typ) {
			if m.Decl == nil || !ctx.sameFeature(m, member, typ) {
				continue
			}
			if behavior, ok := lower.StartableBehaviorOf(m.Decl); ok {
				return classifierBehaviorDecl{behavior: behavior, member: m}, typ, true
			}
		}
	}
	return classifierBehaviorDecl{}, nil, false
}

// sameFeature reports whether m, a member of typ, is member itself or redefines it.
func (ctx *Context) sameFeature(m, member, typ *symbols.Symbol) bool {
	return m == member || slices.Contains(ctx.redefinedFeatures(m, typ), member)
}

// bindingIndex is member's position among the behaviors typ binds to every object of
// it, or -1 for one the type only declares for its objects to start.
func (ctx *Context) bindingIndex(typ, member *symbols.Symbol) int {
	for i, decl := range ctx.classifierBehaviorsOf(typ) {
		if decl.member == member {
			return i
		}
	}
	return -1
}

// startableDeclaration is the behavior member binds on the object, whether its type runs
// it on every object or declares it for a start to run.
func (ctx *Context) startableDeclaration(inst *Instance, member *symbols.Symbol) (classifierBehaviorDecl, bool) {
	decl, _, ok := ctx.startableBehaviorOf(inst, member)
	return decl, ok
}
