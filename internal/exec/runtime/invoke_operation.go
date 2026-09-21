package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// OperationArguments is an invocation's argument list (KerML 8.2.5.8.3): either
// positional, bound to the operation's `in` and `inout` parameters in declaration
// order, or named. A list giving both is refused.
type OperationArguments struct {
	Positional []Value
	Named      map[string]Value
}

// InvokeOperation runs a behavior the object's type owns with the object as the
// performer: what the body reads and writes is that object's feature values, and
// what it sends and accepts carries that object's identity. Arguments bind to the
// operation's `in` and `inout` parameters by name.
func (ctx *Context) InvokeOperation(inst *Instance, name string, args map[string]Value) (map[string]Value, error) {
	return ctx.InvokeOperationWith(inst, name, OperationArguments{Named: args})
}

// InvokeOperationWith is InvokeOperation taking either argument list form.
func (ctx *Context) InvokeOperationWith(inst *Instance, name string, args OperationArguments) (map[string]Value, error) {
	defer ctx.beginRun()()

	if inst == nil {
		return nil, fmt.Errorf("%w: no object to perform %s", ErrNoSuchBehavior, name)
	}
	if err := ctx.checkPerformer(inst); err != nil {
		return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
	}
	if len(args.Positional) > 0 && len(args.Named) > 0 {
		return nil, fmt.Errorf("%w: operation %s is invoked with %d positional and %d named argument(s)",
			ErrMixedArguments, name, len(args.Positional), len(args.Named))
	}
	sym, err := ctx.operationOf(inst, name, args)
	if err != nil {
		return nil, err
	}
	inputs, err := operationInputs(ctx.model.semantics.SignatureParametersOf(sym), name, args)
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
		result, err := ctx.invokeCalcNamedShapeOn(shape, inputs, DeclScope(sym), inst)
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
		holds, err := ctx.evaluateConstraintInvocation(sym, DeclScope(sym), inst, inputs)
		if err != nil {
			return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
		}
		return map[string]Value{"result": boolValue(holds)}, nil
	}
	return nil, fmt.Errorf("%w: %s of %s", ErrNotABehavior, name, symbolText(inst.Type))
}

// operationOf resolves the member of the object's type that name invokes — among
// several so named, the one the arguments' values select as a call in the model
// would — and reports a member that states no executable behavior.
func (ctx *Context) operationOf(inst *Instance, name string, args OperationArguments) (*symbols.Symbol, error) {
	member, err := ctx.memberCalled(inst.Type, inst, name, args)
	if err != nil {
		return nil, err
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

// memberCalled is the member of owner that a call of name with args denotes:
// among several so named, the one the arguments' values select as a call in
// the model would, evaluated as self; nil when none is so named.
func (ctx *Context) memberCalled(owner *symbols.Symbol, self *Instance, name string, args OperationArguments) (*symbols.Symbol, error) {
	var candidates []*symbols.Symbol
	for _, candidate := range ctx.model.semantics.MembersOf(owner) {
		if candidate.Name == name {
			candidates = append(candidates, candidate)
		}
	}
	switch len(candidates) {
	case 0:
		return nil, nil
	case 1:
		return candidates[0], nil
	}
	scope := DeclScope(owner)
	ec := NewEvalContextIn(ctx, scope, self)
	typed := make([]semantics.Argument, 0, len(args.Positional)+len(args.Named))
	for _, value := range args.Positional {
		typed = append(typed, ec.valueArgument(value, nil))
	}
	for _, param := range slices.Sorted(maps.Keys(args.Named)) {
		typed = append(typed, ec.valueArgument(args.Named[param], ast.QualifiedNameOf(param)))
	}
	sel := ctx.model.semantics.SelectAmongArguments(scope, candidates, typed, semantics.PerformsOperation)
	if sel.Ambiguous || sel.Called() == nil {
		return nil, ambiguousInvocationError(name, sel.Tied)
	}
	return sel.Called(), nil
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

// operationInputs binds arguments to the operation's input parameters — a positional
// list in signature order, a named one by name — reporting a surplus positional, an
// argument naming no parameter and a parameter left with no value: any would
// otherwise run the body against values the invocation never stated.
func operationInputs(params []semantics.SignatureParameter, name string, args OperationArguments) (map[string]Value, error) {
	named := args.Named
	if len(args.Positional) > 0 {
		if len(args.Positional) > len(params) {
			return nil, fmt.Errorf("%w: operation %s takes %d input parameter(s), got %d argument(s)",
				ErrOperationArity, name, len(params), len(args.Positional))
		}
		named = make(map[string]Value, len(args.Positional))
		for i, value := range args.Positional {
			named[params[i].Name] = value
		}
	}
	inputs := make(map[string]Value, len(named))
	for _, param := range params {
		value, bound := named[param.Name]
		switch {
		case bound:
			inputs[param.Name] = value
		case param.Optional:
		default:
			return nil, fmt.Errorf("%w: parameter %s of operation %s has no argument and no default",
				ErrUnboundParameter, param.Name, name)
		}
	}
	for arg := range named {
		if !bindsParameter(params, arg) {
			return nil, fmt.Errorf("%w: %s is no input parameter of operation %s",
				ErrUnboundParameter, arg, name)
		}
	}
	return inputs, nil
}

// bindsParameter reports whether name is an input parameter an invocation binds.
func bindsParameter(params []semantics.SignatureParameter, name string) bool {
	for _, param := range params {
		if param.Name == name {
			return true
		}
	}
	return false
}
