package queryplan

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// The plan builders construct a program directly, for a query surface that
// is translated into plan operations rather than compiled from a library
// declaration: the operations and their argument names are the library's.

// Literal is a plan literal of the given kind.
func Literal(kind LiteralKind, value string, origin symbols.Origin) Expression {
	return Expression{operation: OperationLiteral, literal: kind, value: value, origin: origin}
}

// StringLiteral is a string literal.
func StringLiteral(value string, origin symbols.Origin) Expression {
	return Literal(LiteralString, value, origin)
}

// BooleanLiteral is a boolean literal.
func BooleanLiteral(value bool, origin symbols.Origin) Expression {
	if value {
		return Literal(LiteralBoolean, "true", origin)
	}
	return Literal(LiteralBoolean, "false", origin)
}

// IntegerLiteral is an integer literal written in decimal.
func IntegerLiteral(value string, origin symbols.Origin) Expression {
	return Literal(LiteralInteger, value, origin)
}

// Sequence concatenates the values of its operands, in order.
func Sequence(origin symbols.Origin, items ...Expression) Expression {
	arguments := make([]Argument, len(items))
	for i, item := range items {
		arguments[i] = Argument{Value: item}
	}
	return Expression{operation: OperationSequence, arguments: arguments, origin: origin}
}

// ParameterRef reads the entry definition's parameter so named.
func ParameterRef(name string, origin symbols.Origin) Expression {
	return Expression{operation: OperationParameter, target: name, origin: origin}
}

// Operate is an operation applied to named arguments, each one of the
// library's parameters for it; an omitted defaulted parameter takes its default.
func Operate(operation Operation, origin symbols.Origin, arguments ...Argument) Expression {
	named := make([]Argument, len(arguments))
	for i, argument := range arguments {
		argument.Named = true
		named[i] = argument
	}
	return Expression{operation: operation, arguments: named, origin: origin}
}

// Arg names one argument of Operate.
func Arg(name string, value Expression) Argument {
	return Argument{Name: name, Named: true, Value: value}
}

// NewProgram is a program of one entry definition over the given parameters,
// whose result is the expression.
func NewProgram(name string, parameters []Parameter, result Parameter, expression Expression, origin symbols.Origin) *Program {
	definition := Definition{
		name:       name,
		parameters: cloneParameters(parameters),
		result:     result,
		expression: expression.clone(),
		origin:     origin,
	}
	return &Program{entry: name, definitions: []Definition{definition}}
}
