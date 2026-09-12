package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// functionValue is a calc read as a value: its lowered invocation interface and
// the environment it was read in, which an invocation of the value runs it against.
// A library calc the runtime implements natively carries that implementation instead.
type functionValue struct {
	shape   *calcShape
	library *libraryFunction
	scope   *symbols.Scope // names the calc's defaults and body resolve against
	self    *Instance      // object the calc's feature names resolve against, nil for none
	// enclosing are the bindings of the behavior body the calc is declared in, as
	// they stood when it was read; nil for a calc reading none of them.
	enclosing []frame
}

// function is the payload of a ValFunction; nil for every other kind.
func (v Value) function() *functionValue {
	if v.Kind != ValFunction {
		return nil
	}
	fn, _ := v.ref.(*functionValue)
	return fn
}

// Function is the calc a ValFunction is a value of — the definition or usage it
// was read from; nil for every other kind.
func (v Value) Function() *symbols.Symbol {
	if fn := v.function(); fn != nil && fn.shape != nil {
		return fn.shape.Sym
	}
	return nil
}

// FunctionName is the qualified name of the calc a ValFunction is a value of, as
// the wire and diagnostics spell it.
func (v Value) FunctionName() string {
	if fn := v.function(); fn != nil && fn.shape != nil {
		return fn.shape.Name
	}
	return "<unknown function>"
}

// FunctionSelf is the object a ValFunction's feature names resolve against, nil
// for a function closing over none.
func (v Value) FunctionSelf() *Instance {
	if fn := v.function(); fn != nil {
		return fn.self
	}
	return nil
}

// FunctionClosesOverBody reports a ValFunction closing over the bindings of the
// behavior body its calc is declared in, which nothing outside that run can rebuild.
func (v Value) FunctionClosesOverBody() bool {
	fn := v.function()
	return fn != nil && len(fn.enclosing) > 0
}

// functionRun identifies the behavior run a ValFunction closes over (frame.run), so
// every read of the calc within that run is one function; 0 for one closing over none.
func (v Value) functionRun() int64 {
	fn := v.function()
	if fn == nil || len(fn.enclosing) == 0 {
		return 0
	}
	return fn.enclosing[len(fn.enclosing)-1].run
}

// functionValueOf is the value of the calc sym denotes in this environment: its
// lowered shape closed over the scope and object the read resolves against. A
// library calc applied natively is the value of that implementation; one bound
// by an unevaluated argument (a built-in) has no value to pass on.
func (ec *EvalContext) functionValueOf(sym *symbols.Symbol) (Value, error) {
	if _, builtin := ec.ctx.builtinFor(sym); builtin {
		return Value{}, fmt.Errorf("%w: %s binds its arguments unevaluated and cannot be read as a value",
			ErrNotAFunction, ec.ctx.qualifiedSymbolName(sym))
	}
	shape, err := ec.ctx.calcInterfaceOf(sym)
	if err != nil {
		return Value{}, err
	}
	fn := &functionValue{shape: shape, scope: ec.scope, self: ec.self}
	fn.library, _ = ec.ctx.libraryFunctionFor(sym)
	if shape.closesOverBody() {
		fn.enclosing = snapshotFrames(ec.enclosingRun(shape))
	}
	return Value{Kind: ValFunction, ref: fn}, nil
}

// isCalcDefSymbol reports a symbol declaring a calc definition or KerML function.
func isCalcDefSymbol(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolCalcDef
}

// readsAsFunction reports a calc a bare read of its name denotes as a function: a
// calc definition, or a calc usage with an input no read could supply, which
// therefore computes no result to read — whether or not it has a body yet.
func (ctx *Context) readsAsFunction(sym *symbols.Symbol) bool {
	if isCalcDefSymbol(sym) {
		return true
	}
	if !isCalcUsageSymbol(sym) {
		return false
	}
	shape, err := ctx.calcInterfaceOf(sym)
	return err == nil && shape.hasUnsuppliedInput()
}

// hasUnsuppliedInput reports an input parameter no read of the calc could bind:
// neither an argument, a default nor an omitted optional supplies it.
func (shape *calcShape) hasUnsuppliedInput() bool {
	for i := range shape.Params {
		param := &shape.Params[i]
		if param.Default == nil && !param.IsSubject && !param.optional() {
			return true
		}
	}
	return false
}

// FunctionValue is the function a read of the declaration sym denotes — a calc
// definition, or a calc usage with an input no read could supply — closed over
// its own scope and no object; false when sym is no calc read as one.
func (ctx *Context) FunctionValue(sym *symbols.Symbol) (Value, bool, error) {
	if !ctx.readsAsFunction(sym) {
		return Value{}, false, nil
	}
	val, err := NewEvalContextIn(ctx, sym.OwnerScope, nil).functionValueOf(sym)
	return val, true, err
}

// calcAsValue is the function value a bare read of sym denotes, false when sym
// is no calc read as one.
func (ec *EvalContext) calcAsValue(sym *symbols.Symbol) (Value, bool, error) {
	if !ec.ctx.readsAsFunction(sym) {
		return Value{}, false, nil
	}
	val, err := ec.functionValueOf(sym)
	return val, true, err
}

// boundFunction is the value the calc-typed feature callee holds in this
// environment — a parameter bound by argument, or a feature of the bound object —
// which an invocation of callee applies; false when nothing here binds it.
func (ec *EvalContext) boundFunction(callee *symbols.Symbol, qn *ast.QualifiedName) (Value, bool, error) {
	if qn == nil || len(qn.Parts) == 0 || !isCalcUsageSymbol(callee) {
		return Value{}, false, nil
	}
	if len(qn.Parts) > 1 {
		return ec.qualifiedBoundFunction(callee, qn)
	}
	if qn.Global {
		return Value{}, false, nil
	}
	name := qn.Parts[0].Text
	if val, ok := ec.Lookup(name); ok {
		return val, true, nil
	}
	// A calc-typed feature the element being evaluated binds (`in calc :>> f = g`)
	// is called as the function it is bound to.
	if val, ok, err := ec.valuedFeatureValue(name); ok {
		if err != nil {
			return Value{}, true, err
		}
		if val.Kind == ValFunction {
			return val, true, nil
		}
	}
	if ec.self != nil && ec.selfFeatureInScope(name) {
		val, ok, err := ec.selfFeatureValue(name)
		if err != nil {
			return Value{}, true, err
		}
		if ok && val.Kind == ValFunction {
			return val, true, nil
		}
	}
	return Value{}, false, nil
}

// qualifiedBoundFunction reads what the innermost run of the qualifying calc
// (`Apply::f(2.0)`), or of one specializing it, bound the calc-typed callee to.
func (ec *EvalContext) qualifiedBoundFunction(callee *symbols.Symbol, qn *ast.QualifiedName) (Value, bool, error) {
	qualifier, ok := ec.ctx.readQualified(ec.scope, qn).Part(len(qn.Parts) - 2)
	if !ok {
		return Value{}, false, nil
	}
	val, ok := ec.frameFeatureValue(qualifier, callee)
	return val, ok, nil
}

// checkFunction refuses a value bound to a calc usage parameter that is no
// function; an omitted optional parameter holds null.
func (param *calcParameter) checkFunction(value *Value, what func() string) error {
	if !param.IsCalc || value.Kind == ValFunction || value.Kind == ValNull {
		return nil
	}
	return fmt.Errorf("%s: %w: %s is %s, not a function",
		what(), ErrNotAFunction, FormatValue(*value), describeValue(*value))
}

// invokeFunction applies a value called as a function to args through the calc
// invocation path: a value that is no function is refused with a typed error.
func (ec *EvalContext) invokeFunction(callee string, val Value, args calcArgs) (Value, error) {
	fn := val.function()
	if fn == nil || fn.shape == nil {
		return Value{}, fmt.Errorf("%w: %s is %s, not a function", ErrNotAFunction, callee, describeValue(val))
	}
	var result Value
	var err error
	if fn.library != nil {
		result, err = fn.library.invoke(ec.ctx, args)
	} else {
		result, err = ec.ctx.invokeCalcShapeIn(fn.shape, args, fn.scope, fn.self, fn.enclosing)
	}
	ec.ctx.evaluations.record(fn, args, result, err)
	return result, err
}
