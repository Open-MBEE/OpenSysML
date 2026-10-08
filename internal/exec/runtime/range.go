package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A range is not a value kind of its own: the Kernel Function Library declares
// `..` as a function whose result is a collection — abstractly over DataValue
// (DataFunctions::'..', `return : DataValue[0..*] ordered`) and concretely over
// the integers (IntegerFunctions::'..', `return : Integer[0..*]`). So `1..5` is
// the ordered sequence (1, 2, 3, 4, 5), which is what the library's own use of
// it needs: SequenceFunctions::subsequence is
// `(startIndex..endIndex)->collect {in i; seq#(i)}`, and its `tail(seq)` case —
// `subsequence(seq, 2)` of a one-element sequence — reaches `2..1`, so a range
// whose lower bound exceeds its upper one is the empty sequence the library's
// `[0..*]` result allows rather than an error.

// evalRange evaluates `lower..upper` (IntegerFunctions::'..').
func (ec *EvalContext) evalRange(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("%w: '..' requires 2 operands, got %d", ErrCalcArity, len(n.Operands))
	}
	lower, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	upper, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	return ec.rangeSequence("'..'", lower, upper)
}

// rangeSequence builds the ordered sequence of integers from lower to upper.
// Every element costs a step and an element of the budgets, so a range too wide
// to hold is reported before it is held.
func (ec *EvalContext) rangeSequence(op string, lowerVal, upperVal Value) (Value, error) {
	if _, open := undeterminedIn(lowerVal, upperVal); open {
		return undeterminedOf(openRange(), lowerVal, upperVal), nil
	}
	lower, err := rangeBound(op, "lower", lowerVal)
	if err != nil {
		return Value{}, err
	}
	upper, err := rangeBound(op, "upper", upperVal)
	if err != nil {
		return Value{}, err
	}
	// A descending range names no integer: the library's own subsequence reaches
	// one and expects nothing from it.
	if semantics.CompareInt(lower, upper) > 0 {
		return ec.newSequence(nil)
	}
	lo, loSmall := lower.Int64()
	hi, hiSmall := upper.Int64()
	if !loSmall || !hiSmall {
		return ec.bigRangeSequence(lower, upper)
	}
	return ec.smallRangeSequence(lo, hi)
}

// smallRangeSequence is rangeSequence between bounds within int64.
func (ec *EvalContext) smallRangeSequence(lower, upper int64) (Value, error) {
	// A model-supplied width can overflow or exceed what is allocatable, so it only
	// hints at the capacity; the budgets below bound the sequence.
	const maxHint = 4096
	hint := upper - lower + 1
	if hint <= 0 || hint > maxHint {
		hint = maxHint
	}
	elements := make([]Value, 0, hint)
	for i := lower; ; i++ {
		if err := ec.ctx.incrementStep(); err != nil {
			return Value{}, err
		}
		if err := ec.ctx.chargeElements(1); err != nil {
			return Value{}, err
		}
		elements = append(elements, integerValue(i))
		if i == upper {
			break
		}
	}
	return sequenceOf(elements), nil // charged element by element above
}

// bigRangeSequence is rangeSequence where a bound is beyond int64: the elements
// are as exact, and as charged, as within it.
func (ec *EvalContext) bigRangeSequence(lower, upper semantics.Value) (Value, error) {
	var elements []Value
	one := semantics.IntValue(1)
	for i := lower; ; {
		if err := ec.ctx.incrementStep(); err != nil {
			return Value{}, err
		}
		if err := ec.ctx.chargeElements(1); err != nil {
			return Value{}, err
		}
		elements = append(elements, Value{Kind: ValConst, Const: i})
		if semantics.CompareInt(i, upper) == 0 {
			break
		}
		next, err := semantics.IntArith(ast.OpAdd, i, one, ec.ctx.maxIntegerBits)
		if err != nil {
			return Value{}, integerSizeHint(err)
		}
		i = next
	}
	return sequenceOf(elements), nil // charged element by element above
}

// rangeBound reads one bound of a range: IntegerFunctions::'..' declares both
// `Integer[1]`, so a Real bound does not conform and is reported rather than
// truncated to the integer it is nearest.
func rangeBound(op, which string, val Value) (semantics.Value, error) {
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		return semantics.Value{}, fmt.Errorf(
			"%w: %s requires Integer bounds (IntegerFunctions::'..' declares in %s: Integer[1]), got %s",
			ErrTypeMismatch, op, which, describeValue(val),
		)
	}
	return val.Const, nil
}

// rangeBuiltin is the range function op ('..' at any level of the library)
// called as a function.
func rangeBuiltin(op string) builtinFunc {
	return func(ec *EvalContext, args []Value) (Value, error) {
		if err := checkArity(op, args, 2); err != nil {
			return Value{}, err
		}
		return ec.rangeSequence(op, args[0], args[1])
	}
}
