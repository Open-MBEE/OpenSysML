package queryexec

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// computedColumn is one planned columns entry of a projection: a
// Column(name, expression), a PropertyColumn when property is set, or a
// RelatedColumn when related is set.
type computedColumn struct {
	name       string
	expression queryplan.Expression
	property   string
	related    *relatedColumn
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
	case queryplan.OperationColumn, queryplan.OperationPropertyColumn, queryplan.OperationRelatedColumn:
		elements = []queryplan.Expression{value}
	default:
		return nil, e.invalidArgument(project, "columns", string(value.Operation()))
	}
	columns := make([]computedColumn, 0, len(elements))
	for _, element := range elements {
		column := computedColumn{name: element.Target(), origin: element}
		switch element.Operation() {
		case queryplan.OperationColumn:
			expression, ok := argumentValue(element, "expression")
			if !ok {
				return nil, e.invalidArgument(project, "columns", column.name)
			}
			column.expression = expression
		case queryplan.OperationPropertyColumn:
			property, err := e.propertyColumnProperty(element, column.name)
			if err != nil {
				return nil, columnScoped(err, column.name)
			}
			column.property = property
		case queryplan.OperationRelatedColumn:
			related, err := e.relatedColumnOf(element)
			if err != nil {
				return nil, err
			}
			column.related = related
		default:
			return nil, e.invalidArgument(project, "columns", string(element.Operation()))
		}
		columns = append(columns, column)
	}
	return columns, nil
}

// propertyColumnProperty reads a PropertyColumn's property: one non-empty
// string, or the column's name when omitted or null.
func (e *executor) propertyColumnProperty(column queryplan.Expression, name string) (string, error) {
	if !hasArgument(column, "property") {
		return name, nil
	}
	value, err := e.argument(column, "property")
	if err != nil {
		return "", err
	}
	switch len(value.values) {
	case 0:
		return name, nil
	case 1:
	default:
		return "", e.invalidArgument(column, "property", strconv.Itoa(len(value.values)))
	}
	property, ok := value.values[0].String()
	if !ok {
		return "", e.invalidArgument(column, "property", string(value.values[0].Kind()))
	}
	if property == "" {
		return "", e.invalidArgument(column, "property", property)
	}
	return property, nil
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
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	if column.related != nil {
		return e.evaluateRelatedCell(column, row)
	}
	if column.property != "" {
		values, present, err := e.propertyValues(row, column.property)
		if err != nil {
			return nil, e.unevaluable(column.origin, column.property, row, err)
		}
		tracker.record(column.property, present)
		return values, nil
	}
	values, err := e.evaluateColumnExpression(column.expression, column.name, row, tracker)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, e.columnError(
			ErrorColumnAbsent, column.name, row, column.expression.Origin(), "", "")
	}
	if len(values) > 1 {
		return nil, e.columnError(
			ErrorColumnCardinality, column.name, row, column.expression.Origin(),
			"", strconv.Itoa(len(values)))
	}
	return values, nil
}

func (e *executor) evaluateColumnExpression(
	expression queryplan.Expression,
	column string,
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	switch expression.Operation() {
	case queryplan.OperationRowProperty:
		return e.rowPropertyValues(expression, row, tracker)
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
		return e.evaluateColumnOperator(expression, column, row, tracker)
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

// rowPropertyValues evaluates a row property: metadata features read the query's
// bookkeeping; declared features read the object, verdict, state, event or
// element the row carries, conforming to the property's declaring type.
func (e *executor) rowPropertyValues(
	expression queryplan.Expression,
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	property := expression.Target()
	_, declaring := expression.Literal()
	if declaringIsElement(declaring) {
		values, present, err := e.propertyValues(row, property)
		if err != nil {
			return nil, e.unevaluable(expression, property, row, err)
		}
		tracker.record(property, present)
		return values, nil
	}
	if inst, _, isObject := row.Object(); isObject {
		return e.objectRowValues(expression, property, declaring, row, inst)
	}
	var values []Value
	var done bool
	var err error
	row, values, done, err = e.carrierRowValues(expression, property, declaring, row)
	if done {
		return values, err
	}
	sym, _ := row.Element()
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
	declared, _, err := e.declaredFeatureValues(sym, property)
	if err != nil {
		return nil, e.unevaluable(expression, property, ElementValue(sym), err)
	}
	return declared, nil
}

// objectRowValues reads a declared or metaclass feature of an object row.
func (e *executor) objectRowValues(
	expression queryplan.Expression,
	property, declaring string,
	row Value,
	inst *runtime.Instance,
) ([]Value, error) {
	if isMetaclassFQN(declaring) {
		decl := objectDeclaration(inst)
		if !e.context.Model.MetaclassConforms(decl, declaring) {
			return nil, nil
		}
		return e.reflectiveFeatureValues(expression, property, decl)
	}
	if !e.objectConformsTo(inst, declaring) {
		return nil, nil
	}
	values, _, err := e.objectFeatureValues(row, property)
	if err != nil {
		return nil, e.unevaluable(expression, property, row, err)
	}
	return values, nil
}

// carrierRowValues reads a record row's own property when declaring names the
// record's type, else unwraps the element the record carries; done reports the
// property was evaluated (or the record carries nothing) rather than unwrapped.
func (e *executor) carrierRowValues(
	expression queryplan.Expression,
	property, declaring string,
	row Value,
) (Value, []Value, bool, error) {
	if verdict, isVerdict := row.Verdict(); isVerdict {
		return e.verdictRowValues(expression, property, declaring, row, verdict)
	}
	if state, isState := row.State(); isState {
		return e.stateRowValues(expression, property, declaring, row, state)
	}
	if event, isEvent := row.Event(); isEvent {
		return e.eventRowValues(expression, property, declaring, row, event)
	}
	return row, nil, false, nil
}

// verdictRowValues reads a verdict's own property, or unwraps the assertion.
func (e *executor) verdictRowValues(
	expression queryplan.Expression, property, declaring string, row Value, verdict Verdict,
) (Value, []Value, bool, error) {
	if declaring != verdictFQN {
		return ElementValue(verdict.Assertion()), nil, false, nil
	}
	values, _, err := e.verdictPropertyValues(row, property)
	if err != nil {
		return row, nil, true, e.unevaluable(expression, property, row, err)
	}
	return row, values, true, nil
}

// stateRowValues reads a state's own property, or unwraps its declaration.
func (e *executor) stateRowValues(
	expression queryplan.Expression, property, declaring string, row Value, state State,
) (Value, []Value, bool, error) {
	if declaring == stateFQN {
		values, _, err := e.statePropertyValues(row, property)
		if err != nil {
			return row, nil, true, e.unevaluable(expression, property, row, err)
		}
		return row, values, true, nil
	}
	if state.symbol == nil {
		return row, nil, true, nil
	}
	return ElementValue(state.symbol), nil, false, nil
}

// eventRowValues reads an event's own property, or unwraps its behavior.
func (e *executor) eventRowValues(
	expression queryplan.Expression, property, declaring string, row Value, event Event,
) (Value, []Value, bool, error) {
	if declaring == eventFQN {
		values, _, err := e.eventPropertyValues(row, property)
		if err != nil {
			return row, nil, true, e.unevaluable(expression, property, row, err)
		}
		return row, values, true, nil
	}
	if event.Behavior() == nil {
		return row, nil, true, nil
	}
	return ElementValue(event.Behavior()), nil, false, nil
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
		return nil, e.featureError(expression, property, ElementValue(sym))
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := filterValue(value, sym)
		if !ok {
			return nil, e.featureError(expression, property, ElementValue(sym))
		}
		result = append(result, converted)
	}
	return result, nil
}

// objectConformsTo reports whether an object is of a feature's declaring type.
func (e *executor) objectConformsTo(inst *runtime.Instance, declaring string) bool {
	for _, target := range e.context.Index.LookupQualified(declaring) {
		if e.objectConforms(inst, target) {
			return true
		}
	}
	return false
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
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	_, operator := expression.Literal()
	operands := expression.Arguments()
	if operator == "??" {
		left, err := e.evaluateColumnExpression(operands[0].Value, column, row, tracker)
		if err != nil {
			return nil, err
		}
		if len(left) > 0 {
			return left, nil
		}
		return e.evaluateColumnExpression(operands[1].Value, column, row, tracker)
	}
	values := make([]Value, len(operands))
	for i, operand := range operands {
		operandValues, err := e.evaluateColumnExpression(operand.Value, column, row, tracker)
		if err != nil {
			return nil, err
		}
		if len(operandValues) != 1 {
			return nil, e.columnError(ErrorColumnOperand, column, row, operand.Value.Origin(),
				operator, strconv.Itoa(len(operandValues)))
		}
		values[i] = operandValues[0]
	}
	result, err := e.applyColumnOperator(expression, column, row, operator, values)
	if err != nil {
		return nil, err
	}
	return []Value{valueAt(result, expression.Origin())}, nil
}

func (e *executor) applyColumnOperator(
	expression queryplan.Expression,
	column string,
	row Value,
	operator string,
	values []Value,
) (Value, error) {
	mismatch := func() error {
		kinds := string(values[0].Kind())
		if len(values) == 2 {
			kinds += " and " + string(values[1].Kind())
		}
		return e.columnError(ErrorColumnOperandType, column, row, expression.Origin(), operator, kinds)
	}
	if hasQuantity(values) {
		return e.applyQuantityOperator(expression, column, row, operator, values, mismatch)
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
			realVal, _ := values[0].Real()
			if operator == "-" {
				realVal = -realVal
			}
			return RealValue(realVal), nil
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
					ErrorColumnDivisionByZero, column, row, expression.Origin(), operator, "")
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
				ErrorColumnDivisionByZero, column, row, expression.Origin(), operator, "")
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
	row Value,
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
	// The runtime's operators, so a point on a measurement scale computes as evaluation does.
	reader := e.derived.get(e.context)
	var result semantics.Quantity
	var err error
	if len(operands) == 1 {
		result, err = reader.QuantityUnary(op, operands[0])
	} else {
		result, err = reader.QuantityBinary(op, operands[0], operands[1])
	}
	switch {
	case err == nil:
	case errors.Is(err, semantics.ErrDivisionByZero):
		return Value{}, e.columnError(ErrorColumnDivisionByZero, column, row, expression.Origin(), operator, "")
	case errors.Is(err, semantics.ErrIncommensurableUnits):
		units := operands[0].Unit.String() + " and " + operands[1].Unit.String()
		return Value{}, e.columnError(ErrorColumnIncommensurable, column, row, expression.Origin(), operator, units)
	case errors.Is(err, semantics.ErrQuantityOperand):
		return Value{}, mismatch()
	default:
		return Value{}, e.columnError(ErrorColumnArithmetic, column, row, expression.Origin(), operator, err.Error())
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
		realVal, _ := value.Real()
		return semantics.Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: realVal}, Unit: semantics.UnitOne()}, true
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
	realVal, _ := value.Real()
	return realVal
}

func (e *executor) columnError(
	kind ErrorKind,
	column string,
	row Value,
	origin symbols.Origin,
	operator string,
	actual string,
) error {
	return &Error{
		Kind:      kind,
		Query:     e.definition.Name(),
		Operation: queryplan.OperationProject,
		Property:  column,
		Target:    rowTarget(row),
		Parameter: operator,
		Actual:    actual,
		Origin:    origin,
	}
}
