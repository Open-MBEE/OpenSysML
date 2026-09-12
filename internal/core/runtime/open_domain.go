package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The library types an operator's domain is spelled in. An operand read from a
// feature typed outside all of them cannot be a value the operator is defined for,
// whether or not the model gives it one.
const (
	numericalValueTypeFQN = "ScalarValues::NumericalValue"
	stringTypeFQN         = "ScalarValues::String"
	booleanTypeFQN        = "ScalarValues::Boolean"
)

// openOperandType is the type the model declares for an open operand read from a
// feature; nil for a derived one or a feature declaring no type.
func (ctx *Context) openOperandType(val Value) *symbols.Symbol {
	u := val.Undetermined()
	if u == nil || u.feature == nil {
		return nil
	}
	return ctx.extractType(u.feature)
}

// describeOpenOperand describes an operand for a diagnostic, an open one by the
// type its feature declares and the count it certainly holds, e.g. "an undetermined
// String" or "an undetermined String sequence".
func (ctx *Context) describeOpenOperand(val Value) string {
	if val.Kind != ValUndetermined {
		return describeOperand(val)
	}
	typ := ctx.openOperandType(val)
	several := certainlySeveral(val)
	switch {
	case typ != nil && several:
		return "an undetermined " + typ.Name + " sequence"
	case typ != nil:
		return "an undetermined " + typ.Name
	case several:
		return "an undetermined sequence"
	}
	return describeOperand(val)
}

// certainlySeveral reports whether the values of val number more than one, so no
// value of it is the one a scalar operand holds.
func certainlySeveral(val Value) bool {
	lower := countOf(val).Lower
	return lower.Known && (lower.Infinite || lower.Value > 1)
}

// openOperandAdmits reports whether an open operand may hold one value of one of the
// library types accepts name. A determined operand is not judged here.
func (ctx *Context) openOperandAdmits(val Value, accepts ...string) bool {
	if certainlySeveral(val) {
		return false
	}
	for _, fqn := range accepts {
		if ctx.openValueMayBe(val, ctx.librarySymbol(fqn)) {
			return true
		}
	}
	return false
}

// openValueMayBe reports whether an open value may hold a value of typ: its declared
// type is unknown, or typ is unknown, or either conforms to the other.
func (ctx *Context) openValueMayBe(val Value, typ *symbols.Symbol) bool {
	declared := ctx.openOperandType(val)
	return declared == nil || typ == nil || ctx.modelConforms(declared, typ) || ctx.modelConforms(typ, declared)
}

// openBoolOperand rejects an open operand whose declared type admits no Boolean,
// what naming its place in the operator.
func (ctx *Context) openBoolOperand(what string, val Value) error {
	if val.Kind != ValUndetermined || ctx.openOperandAdmits(val, booleanTypeFQN) {
		return nil
	}
	return fmt.Errorf("%w: %s must be Boolean, got %s", ErrTypeMismatch, what, ctx.describeOpenOperand(val))
}

// openNumericIndex rejects an open index whose declared type admits no number.
func (ctx *Context) openNumericIndex(op string, val Value) error {
	if val.Kind != ValUndetermined || ctx.openOperandAdmits(val, numericalValueTypeFQN) {
		return nil
	}
	return fmt.Errorf("%w: %s requires an Integer index, got %s", ErrTypeMismatch, op, ctx.describeOpenOperand(val))
}

// openUnaryOperand rejects an open operand of the unary operator op whose declared
// type admits nothing the operator is defined for.
func (ctx *Context) openUnaryOperand(op ast.OperatorKind, val Value) error {
	if op == ast.OpNot {
		if ctx.openOperandAdmits(val, booleanTypeFQN) {
			return nil
		}
		return fmt.Errorf("%w: logical not requires bool operand, got %s", ErrTypeMismatch, ctx.describeOpenOperand(val))
	}
	if ctx.openOperandAdmits(val, numericalValueTypeFQN, arrayTypeFQN) {
		return nil
	}
	return fmt.Errorf("%w: unary '%s' requires numeric operand, got %s", ErrTypeMismatch, op, ctx.describeOpenOperand(val))
}

// openBinaryOperands rejects an open operand of the binary operator op whose
// declared type admits no value that could pair with the other operand: domain
// gives the types acceptable beside a determined operand, and whether that operand
// alone puts the operation outside the operator's definition.
func (ctx *Context) openBinaryOperands(
	op ast.OperatorKind, left, right Value, span source.Span,
	domain func(other Value) (accepts []string, defined bool),
) error {
	for i, operand := range []Value{left, right} {
		if operand.Kind != ValUndetermined {
			continue
		}
		other := []Value{right, left}[i]
		accepts, defined := domain(other)
		if defined && ctx.openOperandAdmits(operand, ctx.admittedBy(other, accepts)...) {
			continue
		}
		return &OperandTypeError{
			Op:    op.String(),
			Left:  ctx.describeOpenOperand(left),
			Right: ctx.describeOpenOperand(right),
			Span:  span,
		}
	}
	return nil
}

// admittedBy narrows accepts to the types an open operand may hold; a determined
// operand narrows nothing, the domain having judged it already.
func (ctx *Context) admittedBy(val Value, accepts []string) []string {
	if val.Kind != ValUndetermined {
		return accepts
	}
	var kept []string
	for _, fqn := range accepts {
		if ctx.openOperandAdmits(val, fqn) {
			kept = append(kept, fqn)
		}
	}
	return kept
}

// arithmeticDomain is the types an open operand of op may hold beside other: a
// String pairs only with a String under `+`, a number with a number or an array.
func arithmeticDomain(op ast.OperatorKind) func(other Value) ([]string, bool) {
	return func(other Value) ([]string, bool) {
		switch other.Kind {
		case ValString:
			return []string{stringTypeFQN}, op == ast.OpAdd
		case ValComplex, ValQuantity, ValVector, ValVectorQuantity, ValTensorQuantity, ValMeasurementRef,
			ValCoordinateFrame, ValArray:
			return []string{numericalValueTypeFQN, arrayTypeFQN}, true
		case ValConst:
			if other.Const.IsNumeric() {
				return []string{numericalValueTypeFQN, arrayTypeFQN}, true
			}
		}
		accepts := []string{numericalValueTypeFQN, arrayTypeFQN}
		if op == ast.OpAdd {
			accepts = append(accepts, stringTypeFQN)
		}
		return accepts, true
	}
}

// comparisonDomain is the types an open operand of an ordering may hold beside
// other: a String orders against a String, a number or quantity against a number,
// and a Boolean or Complex against nothing.
func comparisonDomain(other Value) ([]string, bool) {
	switch other.Kind {
	case ValString:
		return []string{stringTypeFQN}, true
	case ValConst:
		return []string{numericalValueTypeFQN}, other.Const.IsNumeric()
	case ValQuantity:
		return []string{numericalValueTypeFQN}, true
	case ValComplex:
		return nil, false
	}
	return []string{numericalValueTypeFQN, stringTypeFQN}, true
}
