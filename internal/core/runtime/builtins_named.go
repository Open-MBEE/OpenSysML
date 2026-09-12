package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// registerNamedOperatorBuiltins adds the function-call forms of the operators
// whose results are collections or whose arguments are expressions, and the
// aggregations that take an explicit identity element.
func registerNamedOperatorBuiltins() {
	// BaseFunctions declares '#' and ',' over any sequence; ',' is
	// SequenceFunctions' union, '#' its index when given one index.
	builtins["BaseFunctions::#"] = builtinBaseIndex
	builtins["BaseFunctions::,"] = builtinSequenceConcat
	// CollectionFunctions::'==' is `col1.elements->equals(col2.elements)`.
	builtins["CollectionFunctions::=="] = builtinCollectionEquals

	// The range, declared abstractly over DataValue and ScalarValue and
	// concretely over Integer; every level yields the integer sequence.
	builtins["DataFunctions::.."] = rangeBuiltin("DataFunctions::'..'")
	builtins["ScalarFunctions::.."] = rangeBuiltin("ScalarFunctions::'..'")

	builtins["ControlFunctions::if"] = builtinControlIf
	builtins["ControlFunctions::??"] = builtinControlNullCoalesce
	builtins["ControlFunctions::and"] = builtinControlLogical(ast.OpConditionalAnd)
	builtins["ControlFunctions::or"] = builtinControlLogical(ast.OpConditionalOr)
	builtins["ControlFunctions::implies"] = builtinControlLogical(ast.OpImplies)

	builtins["NumericalFunctions::sum0"] = builtinNumericalSum0
	builtins["NumericalFunctions::product1"] = builtinNumericalProduct1
}

// evalDeferred evaluates an argument bound to an `expr` parameter: the
// expression as it was written, or the result of the parameterless body it is
// or denotes.
func (ec *EvalContext) evalDeferred(op string, val Value) (Value, error) {
	if val.Kind != ValExpr {
		return val, nil
	}
	body, ok := val.Expr().(*ast.BodyExpr)
	if !ok {
		denoted, err := ec.evalClosure(val)
		if err != nil || denoted.Kind != ValExpr {
			return denoted, err
		}
		if body, ok = denoted.Expr().(*ast.BodyExpr); !ok {
			return Value{}, fmt.Errorf("%w: %s requires a value or a body expression, got %T", ErrTypeMismatch, op, denoted.Expr())
		}
		val = denoted
	}
	if len(body.Params) != 0 {
		return Value{}, fmt.Errorf("%w: %s evaluates its body with no argument, but it declares %d parameter(s)",
			ErrBodyArity, op, len(body.Params))
	}
	return ec.applyBody(val)
}

// deferredCount is how many values evalDeferred would yield for val, as far as the
// declarations fix that without evaluating it: `[0..*]` where they leave it open.
func (ec *EvalContext) deferredCount(val Value) semantics.Range {
	if val.Kind != ValExpr {
		return countOf(val)
	}
	env := val.exprEnv(ec)
	body, ok := val.Expr().(*ast.BodyExpr)
	if !ok {
		return ec.declaredCount(env.scope, val.Expr())
	}
	if body.Result == nil || env.scope == nil {
		return openRange()
	}
	return ec.declaredCount(symbols.BodyExprScope(env.scope, body), body.Result)
}

// builtinControlIf is ControlFunctions::'if'(test, thenValue, elseValue): the
// selected branch alone is evaluated, and an omitted branch is null.
func builtinControlIf(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::'if'"
	if err := checkArity(op, args, 3); err != nil {
		return Value{}, err
	}
	if args[0].Kind == ValUndetermined {
		return undeterminedOf(ec.deferredCount(args[1]).Covering(ec.deferredCount(args[2])), args[0]), nil
	}
	held, err := boolOperand("test of "+op, args[0])
	if err != nil {
		return Value{}, err
	}
	if held {
		return ec.evalDeferred(op, args[1])
	}
	return ec.evalDeferred(op, args[2])
}

// builtinControlNullCoalesce is ControlFunctions::'??'(firstValue, secondValue),
// which evaluates secondValue only when firstValue is null. secondValue is
// `[0..1]`: omitted, it has no value, so a null firstValue stays null.
func builtinControlNullCoalesce(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::'??'"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	return coalesceNull(args[0], func() (Value, error) { return ec.evalDeferred(op, args[1]) })
}

// builtinControlLogical is ControlFunctions::'and', 'or' or 'implies': the
// first value decides alone where it can, and secondValue is evaluated only
// where it cannot. secondValue is `[0..1]` while the result is `Boolean[1]`, so
// a first value that does not decide reports an omitted second.
func builtinControlLogical(op ast.OperatorKind) builtinFunc {
	name := fmt.Sprintf("ControlFunctions::'%s'", op)
	return func(ec *EvalContext, args []Value) (Value, error) {
		if err := checkArity(name, args, 2); err != nil {
			return Value{}, err
		}
		if args[0].Kind != ValUndetermined {
			l, err := boolOperand("firstValue of "+name, args[0])
			if err != nil {
				return Value{}, err
			}
			if decided, result := shortCircuit(op, l); decided {
				return boolValue(result), nil
			}
		}
		if args[1].Kind == ValNull {
			return Value{}, fmt.Errorf("%w: %s(%s) is decided by secondValue, which was not given; the result is Boolean[1]",
				ErrMultiplicityViolation, name, FormatValue(args[0]))
		}
		second, err := ec.evalDeferred(name, args[1])
		if err != nil {
			return Value{}, err
		}
		return combineBooleanValues(op, args[0], second)
	}
}

// builtinSequenceConcat is BaseFunctions::',', the sequence of seq1's elements
// followed by seq2's.
func builtinSequenceConcat(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("BaseFunctions::','", args, 2); err != nil {
		return Value{}, err
	}
	return ec.concatSequences(args[0], args[1])
}

// builtinNumericalSum0 is NumericalFunctions::sum0(collection, zero):
// `collection->reduce '+' ?? zero`, so an empty collection is the zero given.
func builtinNumericalSum0(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregateWithIdentity("NumericalFunctions::sum0", "zero", args, ast.OpAdd)
}

// builtinNumericalProduct1 is NumericalFunctions::product1(collection, one):
// `collection->reduce '*' ?? one`.
func builtinNumericalProduct1(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregateWithIdentity("NumericalFunctions::product1", "one", args, ast.OpMul)
}

// aggregateWithIdentity folds a collection under op from its first element, and
// answers the identity given for an empty one. The library asserts the identity
// is one (`inv { isZero(zero) }`), so another value is reported.
func (ctx *Context) aggregateWithIdentity(op, param string, args []Value, operator ast.OperatorKind) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	identity := args[1]
	if !isIdentityElement(identity, operator) {
		predicate := "isZero"
		if operator == ast.OpMul {
			predicate = "isUnit"
		}
		return Value{}, fmt.Errorf("%w: %s requires %s(%s), got %s",
			ErrTypeMismatch, op, predicate, param, FormatValue(identity))
	}
	if len(elementsOf(args[0])) == 0 {
		return identity, nil
	}
	return ctx.aggregate(op, args[:1], operator, false)
}

// isIdentityElement reports whether val is the additive (0) or multiplicative
// (1) identity of the numbers this runtime aggregates.
func isIdentityElement(val Value, operator ast.OperatorKind) bool {
	want := 0.0
	if operator == ast.OpMul {
		want = 1
	}
	switch val.Kind {
	case ValConst:
		return val.Const.IsNumeric() && toReal(val.Const) == want
	case ValComplex:
		return val.Complex() == complex(want, 0)
	case ValQuantity:
		return toReal(val.Quantity().Num) == want
	}
	return false
}
