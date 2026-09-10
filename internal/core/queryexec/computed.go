package queryexec

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// computedColumn is one planned Column(name, expression) of a projection.
type computedColumn struct {
	name       string
	expression queryplan.Expression
	origin     queryplan.Expression
}

// computedColumns decodes Project's structural columns argument.
func (e *executor) computedColumns(project, value queryplan.Expression) ([]computedColumn, error) {
	var elements []queryplan.Expression
	switch value.Operation() {
	case queryplan.OperationSequence:
		for _, argument := range value.Arguments() {
			elements = append(elements, argument.Value)
		}
	case queryplan.OperationColumn:
		elements = []queryplan.Expression{value}
	default:
		return nil, e.invalidArgument(project, "columns", string(value.Operation()))
	}
	columns := make([]computedColumn, 0, len(elements))
	for _, element := range elements {
		if element.Operation() != queryplan.OperationColumn {
			return nil, e.invalidArgument(project, "columns", string(element.Operation()))
		}
		column := computedColumn{name: element.Target(), origin: element}
		expression, ok := argumentValue(element, "expression")
		if !ok {
			return nil, e.invalidArgument(project, "columns", column.name)
		}
		column.expression = expression
		columns = append(columns, column)
	}
	return columns, nil
}

// propertyTracker records the row properties a projection read and whether
// each was ever present, so an unknown property stays a typed failure.
type propertyTracker struct {
	order   []string
	present map[string]bool
}

func newPropertyTracker() *propertyTracker {
	return &propertyTracker{present: make(map[string]bool)}
}

func (t *propertyTracker) record(property string, present bool) {
	if _, seen := t.present[property]; !seen {
		t.order = append(t.order, property)
	}
	t.present[property] = t.present[property] || present
}

func (t *propertyTracker) missing() (string, bool) {
	for _, property := range t.order {
		if !t.present[property] {
			return property, true
		}
	}
	return "", false
}

// evaluateColumnCell evaluates one computed column for one row element.
// A failure or absent final result fails the query; ?? defaults absence.
func (e *executor) evaluateColumnCell(
	column computedColumn,
	sym *symbols.Symbol,
	tracker *propertyTracker,
) ([]Value, error) {
	values, err := e.evaluateColumnExpression(column.expression, column.name, sym, tracker)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, e.columnError(
			ErrorColumnAbsent, column.name, sym, column.expression.Origin(), "", "")
	}
	if len(values) > 1 {
		return nil, e.columnError(
			ErrorColumnCardinality, column.name, sym, column.expression.Origin(),
			"", strconv.Itoa(len(values)))
	}
	return values, nil
}

func (e *executor) evaluateColumnExpression(
	expression queryplan.Expression,
	column string,
	sym *symbols.Symbol,
	tracker *propertyTracker,
) ([]Value, error) {
	switch expression.Operation() {
	case queryplan.OperationRowProperty:
		property := expression.Target()
		_, declaring := expression.Literal()
		if declaringIsElement(declaring) {
			values, present, err := e.propertyValues(sym, property)
			if err != nil {
				return nil, e.unevaluable(expression, property, sym, err)
			}
			tracker.record(property, present)
			return values, nil
		}
		if isMetaclassFQN(declaring) {
			if !e.context.Model.MetaclassConforms(sym, declaring) {
				return nil, nil
			}
			return e.reflectiveFeatureValues(expression, property, sym)
		}
		if !e.rowConformsTo(sym, declaring) {
			// The row is unrelated to the declaring type: read as absent so a
			// ?? operator can default it. The feature resolved at planning.
			return nil, nil
		}
		values, _, err := e.declaredFeatureValues(sym, property)
		if err != nil {
			return nil, e.unevaluable(expression, property, sym, err)
		}
		return values, nil
	case queryplan.OperationLiteral:
		value, err := e.evaluateLiteral(expression)
		if err != nil {
			return nil, err
		}
		return value.values, nil
	case queryplan.OperationParameter:
		binding, ok := e.bindings[expression.Target()]
		if !ok {
			return nil, e.errorAt(ErrorMissingBinding, expression)
		}
		return append([]Value(nil), binding.values...), nil
	case queryplan.OperationColumnOperator:
		return e.evaluateColumnOperator(expression, column, sym, tracker)
	default:
		return nil, &Error{
			Kind:      ErrorUnsupportedOperation,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Property:  column,
			Origin:    expression.Origin(),
		}
	}
}

// declaringIsElement reports whether a planned row property was declared on
// Element (or without a declaring type), so it reads query metadata.
func declaringIsElement(declaring string) bool {
	return declaring == "" || declaring == "Element" || declaring == "KerML::Root::Element"
}

// isMetaclassFQN reports whether a declaring type is a reflective metaclass
// of the abstract syntax, which the stdlib declares under KerML and SysML.
func isMetaclassFQN(fqn string) bool {
	return strings.HasPrefix(fqn, "KerML::") || strings.HasPrefix(fqn, "SysML::")
}

// reflectiveFeatureValues reads a metaclass feature (e.g. Type::isAbstract)
// from the row's declaration; an underived one is a typed failure.
func (e *executor) reflectiveFeatureValues(
	expression queryplan.Expression,
	property string,
	sym *symbols.Symbol,
) ([]Value, error) {
	values, ok := e.context.Model.ReflectiveFeatureValues(sym, property)
	if !ok {
		return nil, e.featureError(expression, property, sym)
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := filterValue(value, sym)
		if !ok {
			return nil, e.featureError(expression, property, sym)
		}
		result = append(result, converted)
	}
	return result, nil
}

// rowConformsTo reports whether the row conforms to a feature's declaring
// type; unrelated same-named features read as absent.
func (e *executor) rowConformsTo(sym *symbols.Symbol, declaring string) bool {
	for _, target := range e.context.Index.LookupQualified(declaring) {
		if symbols.SameElement(sym, target) || e.context.Model.Conforms(sym, target) {
			return true
		}
		// An index may hold a distinct Symbol for the same declaration, so
		// compare the supertype chain by declaration, not pointer.
		for _, super := range e.context.Model.AllSupertypes(sym) {
			if symbols.SameElement(super, target) {
				return true
			}
		}
	}
	return false
}

func (e *executor) evaluateColumnOperator(
	expression queryplan.Expression,
	column string,
	sym *symbols.Symbol,
	tracker *propertyTracker,
) ([]Value, error) {
	_, operator := expression.Literal()
	operands := expression.Arguments()
	if operator == "??" {
		left, err := e.evaluateColumnExpression(operands[0].Value, column, sym, tracker)
		if err != nil {
			return nil, err
		}
		if len(left) > 0 {
			return left, nil
		}
		return e.evaluateColumnExpression(operands[1].Value, column, sym, tracker)
	}
	values := make([]Value, len(operands))
	for i, operand := range operands {
		operandValues, err := e.evaluateColumnExpression(operand.Value, column, sym, tracker)
		if err != nil {
			return nil, err
		}
		if len(operandValues) != 1 {
			return nil, e.columnError(ErrorColumnOperand, column, sym, operand.Value.Origin(),
				operator, strconv.Itoa(len(operandValues)))
		}
		values[i] = operandValues[0]
	}
	result, err := e.applyColumnOperator(expression, column, sym, operator, values)
	if err != nil {
		return nil, err
	}
	return []Value{valueAt(result, expression.Origin())}, nil
}

func (e *executor) applyColumnOperator(
	expression queryplan.Expression,
	column string,
	sym *symbols.Symbol,
	operator string,
	values []Value,
) (Value, error) {
	mismatch := func() error {
		kinds := string(values[0].Kind())
		if len(values) == 2 {
			kinds += " and " + string(values[1].Kind())
		}
		return e.columnError(ErrorColumnOperandType, column, sym, expression.Origin(), operator, kinds)
	}
	if hasQuantity(values) {
		return e.applyQuantityOperator(expression, column, sym, operator, values, mismatch)
	}
	if len(values) == 1 {
		// Unary + and - require one numeric operand.
		switch values[0].Kind() {
		case ValueInteger:
			integer, _ := values[0].Integer()
			if operator == "-" {
				integer = -integer
			}
			return IntegerValue(integer), nil
		case ValueReal:
			real, _ := values[0].Real()
			if operator == "-" {
				real = -real
			}
			return RealValue(real), nil
		default:
			return Value{}, mismatch()
		}
	}
	left, right := values[0], values[1]
	if operator == "+" && left.Kind() == ValueString && right.Kind() == ValueString {
		l, _ := left.String()
		r, _ := right.String()
		return StringValue(l + r), nil
	}
	if !arithmeticKind(left.Kind()) || !arithmeticKind(right.Kind()) {
		return Value{}, mismatch()
	}
	if left.Kind() == ValueInteger && right.Kind() == ValueInteger {
		l, _ := left.Integer()
		r, _ := right.Integer()
		switch operator {
		case "+":
			return IntegerValue(l + r), nil
		case "-":
			return IntegerValue(l - r), nil
		case "*":
			return IntegerValue(l * r), nil
		case "/":
			if r == 0 {
				return Value{}, e.columnError(
					ErrorColumnDivisionByZero, column, sym, expression.Origin(), operator, "")
			}
			return IntegerValue(l / r), nil
		}
	}
	l := realOperand(left)
	r := realOperand(right)
	switch operator {
	case "+":
		return RealValue(l + r), nil
	case "-":
		return RealValue(l - r), nil
	case "*":
		return RealValue(l * r), nil
	case "/":
		if r == 0 {
			return Value{}, e.columnError(
				ErrorColumnDivisionByZero, column, sym, expression.Origin(), operator, "")
		}
		return RealValue(l / r), nil
	}
	return Value{}, e.operatorError(expression, operator)
}

func arithmeticKind(kind ValueKind) bool {
	return kind == ValueInteger || kind == ValueReal
}

func hasQuantity(values []Value) bool {
	for _, value := range values {
		if value.Kind() == ValueQuantity {
			return true
		}
	}
	return false
}

// applyQuantityOperator computes with at least one quantity operand under the
// runtime's unit rules: sums need commensurable units, products compose them.
func (e *executor) applyQuantityOperator(
	expression queryplan.Expression,
	column string,
	sym *symbols.Symbol,
	operator string,
	values []Value,
	mismatch func() error,
) (Value, error) {
	op, ok := columnOperatorKind(operator, len(values) == 1)
	if !ok {
		return Value{}, e.operatorError(expression, operator)
	}
	operands := make([]semantics.Quantity, len(values))
	for i, value := range values {
		quantity, ok := quantityOperand(value)
		if !ok {
			return Value{}, mismatch()
		}
		operands[i] = quantity
	}
	var result semantics.Quantity
	var err error
	if len(operands) == 1 {
		result, err = semantics.QuantityUnary(op, operands[0])
	} else {
		result, err = semantics.QuantityBinary(op, operands[0], operands[1])
	}
	if err == nil && (op == ast.OpMul || op == ast.OpDiv) {
		result, err = e.context.Model.CoherentQuantity(result, nil)
	}
	switch {
	case err == nil:
	case errors.Is(err, semantics.ErrDivisionByZero):
		return Value{}, e.columnError(ErrorColumnDivisionByZero, column, sym, expression.Origin(), operator, "")
	case errors.Is(err, semantics.ErrIncommensurableUnits):
		units := operands[0].Unit.String() + " and " + operands[1].Unit.String()
		return Value{}, e.columnError(ErrorColumnIncommensurable, column, sym, expression.Origin(), operator, units)
	case errors.Is(err, semantics.ErrQuantityOperand):
		return Value{}, mismatch()
	default:
		return Value{}, e.columnError(ErrorColumnArithmetic, column, sym, expression.Origin(), operator, err.Error())
	}
	if result.Unit.None() {
		value, ok := constantValue(result.Num)
		if !ok {
			return Value{}, mismatch()
		}
		return value, nil
	}
	return QuantityValue(result), nil
}

// quantityOperand reads a column operand as a quantity, a bare number being one
// in no unit.
func quantityOperand(value Value) (semantics.Quantity, bool) {
	switch value.Kind() {
	case ValueQuantity:
		return value.Quantity()
	case ValueInteger:
		integer, _ := value.Integer()
		return semantics.Quantity{Num: semantics.Value{Kind: semantics.ValInt, Int: integer}, Unit: semantics.UnitOne()}, true
	case ValueReal:
		real, _ := value.Real()
		return semantics.Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: real}, Unit: semantics.UnitOne()}, true
	}
	return semantics.Quantity{}, false
}

func columnOperatorKind(operator string, unary bool) (ast.OperatorKind, bool) {
	switch operator {
	case "+":
		if unary {
			return ast.OpPos, true
		}
		return ast.OpAdd, true
	case "-":
		if unary {
			return ast.OpNeg, true
		}
		return ast.OpSub, true
	case "*":
		return ast.OpMul, true
	case "/":
		return ast.OpDiv, true
	}
	return 0, false
}

func realOperand(value Value) float64 {
	if integer, ok := value.Integer(); ok {
		return float64(integer)
	}
	real, _ := value.Real()
	return real
}

func (e *executor) columnError(
	kind ErrorKind,
	column string,
	sym *symbols.Symbol,
	origin provenance.Origin,
	operator string,
	actual string,
) error {
	return &Error{
		Kind:      kind,
		Query:     e.definition.Name(),
		Operation: queryplan.OperationProject,
		Property:  column,
		Target:    symbols.FQNOf(sym),
		Parameter: operator,
		Actual:    actual,
		Origin:    origin,
	}
}
