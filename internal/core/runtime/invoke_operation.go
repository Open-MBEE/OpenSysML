package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// InvokeOperation runs a behavior the object's type owns with the object as the
// performer: what the body reads and writes is that object's feature values, and
// what it sends and accepts carries that object's identity. Arguments bind to the
// operation's `in` and `inout` parameters by name.
func (ctx *Context) InvokeOperation(inst *Instance, name string, args map[string]Value) (map[string]Value, error) {
	defer ctx.beginRun()()

	if inst == nil {
		return nil, fmt.Errorf("%w: no object to perform %s", ErrNoSuchBehavior, name)
	}
	if err := ctx.checkPerformer(inst); err != nil {
		return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
	}
	sym, err := ctx.operationOf(inst, name)
	if err != nil {
		return nil, err
	}
	inputs, err := operationInputs(ctx.actionParametersOf(sym), name, args)
	if err != nil {
		return nil, err
	}
	switch {
	case isActionSymbol(sym):
		results, err := ctx.ExecuteActionPerformedBy(sym, inst, inputs)
		if err != nil {
			return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
		}

		_, out := parameterNames(ctx.actionParametersOf(sym))
		outputs := make(map[string]Value, len(out))
		for _, param := range out {
			if value, ok := results[param]; ok {
				outputs[param] = value
			}
		}
		return outputs, nil
	case isCalcSymbol(sym):
		shape, err := ctx.calcShapeOf(sym)
		if err != nil {
			return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
		}
		result, err := ctx.invokeCalcNamedShapeOn(shape, inputs, declScope(sym), inst)
		if err != nil {
			return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
		}
		key := "result"
		for _, output := range shape.Outputs {
			if output.IsResult && output.Name != "" {
				key = output.Name
				break
			}
		}
		return map[string]Value{key: result}, nil
	case isConstraintSymbol(sym):
		holds, err := ctx.evaluateConstraintInvocation(sym, declScope(sym), inst, inputs)
		if err != nil {
			return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
		}
		return map[string]Value{"result": boolValue(holds)}, nil
	}
	return nil, fmt.Errorf("%w: %s of %s", ErrNotABehavior, name, symbolText(inst.Type))
}

// operationOf resolves the member of the object's type that name invokes, and
// reports a member that states no executable behavior.
func (ctx *Context) operationOf(inst *Instance, name string) (*symbols.Symbol, error) {
	var member *symbols.Symbol
	for _, candidate := range ctx.model.semantics.MembersOf(inst.Type) {
		if candidate.Name == name {
			member = candidate
			break
		}
	}
	if member == nil {
		return nil, fmt.Errorf("%w: %s of object #%d (type %s)",
			ErrNoSuchBehavior, name, inst.ID, symbolText(inst.Type))
	}
	switch member.Kind {
	case symbols.SymbolActionDef, symbols.SymbolActionUsage:
		return member, nil
	case symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return nil, fmt.Errorf("%w: %s of %s is a state machine, which runs as the object's exhibited machine",
			ErrUnsupportedClassifierBehavior, name, symbolText(inst.Type))
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage:
		return member, nil
	default:
		return nil, fmt.Errorf("%w: %s of %s is a %s",
			ErrNotABehavior, name, symbolText(inst.Type), member.Kind)
	}
}

func isConstraintSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return true
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefConstraint
	case *ast.Usage:
		return decl.Kind == ast.UsageConstraint
	}
	return sym.Kind == symbols.SymbolConstraintDef || sym.Kind == symbols.SymbolConstraintUsage
}

func (ctx *Context) evaluateConstraintInvocation(sym *symbols.Symbol, scope *symbols.Scope, self *Instance, bindings map[string]Value) (bool, error) {
	if err := RequireConstraint(sym); err != nil {
		return false, err
	}
	subject, err := ctx.checkSubject("constraint", sym.Name, sym, self)
	if err != nil {
		return false, err
	}
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:      sym,
		kind:     "constraint",
		what:     "assertion",
		self:     subject.instance,
		bindings: mapFrame(bindings),
		negated:  NegatedDecl(sym),
	}, ctx.conditionsOf(sym, ctx.chainMembers(sym, scope)))
	if errors.Is(err, ErrViolated) {
		return false, nil
	}
	return holds, err
}

// operationInputs binds arguments to the operation's input parameters, reporting
// an argument naming no parameter and a parameter left with no value: either
// would otherwise run the body against values the invocation never stated.
func operationInputs(params []actionParameter, name string, args map[string]Value) (map[string]Value, error) {
	inputs := make(map[string]Value, len(args))
	for _, param := range params {
		if param.Direction == ast.DirOut {
			continue
		}
		value, bound := args[param.Name]
		switch {
		case bound:
			inputs[param.Name] = value
		case param.Optional:
		default:
			return nil, fmt.Errorf("%w: parameter %s of operation %s has no argument and no default",
				ErrUnboundParameter, param.Name, name)
		}
	}
	for arg := range args {
		if !bindsParameter(params, arg) {
			return nil, fmt.Errorf("%w: %s is no input parameter of operation %s",
				ErrUnboundParameter, arg, name)
		}
	}
	return inputs, nil
}

// bindsParameter reports whether name is an input parameter an invocation binds.
func bindsParameter(params []actionParameter, name string) bool {
	for _, param := range params {
		if param.Name == name && param.Direction != ast.DirOut {
			return true
		}
	}
	return false
}
