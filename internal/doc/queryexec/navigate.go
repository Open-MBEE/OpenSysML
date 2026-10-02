package queryexec

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A cell navigates from its row — `row.connectorEnd.type.name` — and
// computes over the collections it reaches with the KerML sequence functions
// (`->size()`, `->select {in e; …}`). Each navigation reads one feature of
// every element reached so far; a body function binds its parameter to each
// element in turn, in a frame the cell's variables are read from.

// cellContext is the computed cell being evaluated: its column, row and
// property tracker, and the variables in scope, innermost first.
type cellContext struct {
	column  string
	row     Value
	tracker *propertyTracker
	frame   *cellFrame
}

// cellFrame binds one variable: the element a collection body is evaluating.
type cellFrame struct {
	name   string
	values []Value
	outer  *cellFrame
}

// enterCell makes column's cell over row the one being evaluated; the result
// restores the previous one.
func (e *executor) enterCell(column computedColumn, row Value, tracker *propertyTracker) func() {
	saved := e.cell
	_, variable := column.origin.Literal()
	e.cell = &cellContext{column: column.name, row: row, tracker: tracker}
	if variable != "" {
		e.cell.frame = &cellFrame{name: variable, values: []Value{row}}
	}
	return func() { e.cell = saved }
}

// bindVariable pushes a body's variable for one element; the result pops it.
func (e *executor) bindVariable(name string, value Value) func() {
	e.cell.frame = &cellFrame{name: name, values: []Value{value}, outer: e.cell.frame}
	return func() { e.cell.frame = e.cell.frame.outer }
}

// evaluateVariable reads a cell variable: the row parameter, or a body's.
func (e *executor) evaluateVariable(expression queryplan.Expression, row Value) ([]Value, error) {
	if e.cell != nil {
		for frame := e.cell.frame; frame != nil; frame = frame.outer {
			if frame.name == expression.Target() {
				return append([]Value(nil), frame.values...), nil
			}
		}
	}
	return nil, e.errorAt(ErrorMissingBinding, expression)
}

// evaluateCellOperand runs a cell expression where a query operation invoked
// inside the cell takes a sequence — RelatedElements(source = row, …).
func (e *executor) evaluateCellOperand(expression queryplan.Expression) (sequence, error) {
	if e.cell == nil {
		return sequence{}, e.errorAt(ErrorMissingBinding, expression)
	}
	values, err := e.evaluateColumnExpression(expression, e.cell.column, e.cell.row, e.cell.tracker)
	if err != nil {
		return sequence{}, err
	}
	return sequence{values: values}, nil
}

// evaluateNavigate reads expression's feature of each element its source
// yields, in source order.
func (e *executor) evaluateNavigate(
	expression queryplan.Expression,
	column string,
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	source, ok := argumentValue(expression, "source")
	if !ok {
		return nil, e.invalidArgument(expression, "source", "")
	}
	values, err := e.evaluateColumnExpression(source, column, row, tracker)
	if err != nil {
		return nil, err
	}
	feature := expression.Target()
	_, declaring := expression.Literal()
	var result []Value
	for _, value := range values {
		step, err := e.navigateValue(expression, value, feature, declaring)
		if err != nil {
			return nil, err
		}
		result = append(result, step...)
	}
	return result, nil
}

// navigateValue reads one feature of one value. A metaclass feature reads
// reflectively from an element of that metaclass, and as absent from any
// other; a feature of unknown declaring type reads what the element declares
// or derives, and as absent where it has neither.
func (e *executor) navigateValue(expression queryplan.Expression, value Value, feature, declaring string) ([]Value, error) {
	if connector, position, ok := value.ConnectorEnd(); ok {
		return e.connectorEndFeature(expression, value, connector, position, feature)
	}
	sym, isElement := value.Element()
	if !isElement {
		if !value.isRow() {
			return nil, e.unevaluable(expression, feature, value, nil)
		}
		values, _, err := e.propertyValues(value, feature)
		if err != nil {
			return nil, e.unevaluable(expression, feature, value, err)
		}
		return values, nil
	}
	if feature == "connectorEnd" && e.context.Model.IsConnectorObjectUsage(sym) {
		return e.connectorEnds(sym), nil
	}
	if declaring != "" {
		if !e.context.Model.MetaclassConforms(sym, declaring) {
			return nil, nil
		}
		values, ok, err := e.reflectiveValues(expression, sym, feature)
		if !ok && err == nil {
			err = e.featureError(expression, feature, value)
		}
		return values, err
	}
	values, present, err := e.propertyValues(value, feature)
	if err != nil {
		return nil, e.unevaluable(expression, feature, value, err)
	}
	if present {
		return values, nil
	}
	values, _, err = e.reflectiveValues(expression, sym, feature)
	return values, err
}

// reflectiveValues reads a metaclass feature of sym: the scalars it derives
// (Element::documentation as the prose of its bodies, as the row property
// reads it), else the elements; ok is false where the model derives neither.
func (e *executor) reflectiveValues(expression queryplan.Expression, sym *symbols.Symbol, feature string) ([]Value, bool, error) {
	elements, ok := e.context.Model.ReflectiveElements(sym, feature)
	if ok {
		values := make([]Value, 0, len(elements))
		for _, element := range elements {
			values = append(values, valueAt(ElementValue(element), ElementValue(sym).Origin()))
		}
		return values, true, nil
	}
	if _, ok := e.context.Model.ReflectiveFeatureValues(sym, feature); ok {
		values, err := e.reflectiveFeatureValues(expression, feature, sym)
		return values, true, err
	}
	return nil, false, nil
}

// connectorEnds is Connector::connectorEnd of a connector usage: one value per
// end, labelled by the attachment it names.
func (e *executor) connectorEnds(connector *symbols.Symbol) []Value {
	paths := e.context.Model.ConnectorEndPaths(connector)
	ends := make([]Value, 0, len(paths))
	for position, path := range paths {
		ends = append(ends, ConnectorEndValue(connector, position, e.endLabel(path)))
	}
	return ends
}

// endLabel spells an end's attachment as the names of the features it walks.
func (e *executor) endLabel(path semantics.ConnectorEndPath) string {
	names := make([]string, 0, len(path.Features))
	for _, feature := range path.Features {
		names = append(names, e.context.Model.EffectiveNameOf(feature))
	}
	if len(names) == 0 {
		return path.Name
	}
	return strings.Join(names, ".")
}

// connectorEndFeature reads a feature of a connector end: its name, owner,
// the features its attachment chains through, or the attached feature's types.
func (e *executor) connectorEndFeature(
	expression queryplan.Expression, value Value, connector *symbols.Symbol, position int, feature string,
) ([]Value, error) {
	paths := e.context.Model.ConnectorEndPaths(connector)
	if position >= len(paths) {
		return nil, e.unevaluable(expression, feature, value, nil)
	}
	path := paths[position]
	elements := func(symbols []*symbols.Symbol) []Value {
		values := make([]Value, 0, len(symbols))
		for _, sym := range symbols {
			values = append(values, valueAt(ElementValue(sym), value.origin))
		}
		return values
	}
	switch feature {
	case "name", "declaredName", "effectiveName":
		if path.Name == "" {
			return nil, nil
		}
		return []Value{valueAt(StringValue(path.Name), value.origin)}, nil
	case "owner", "owningType", "featuringType":
		return elements([]*symbols.Symbol{connector}), nil
	case "chainingFeature":
		return elements(path.Features), nil
	case "type":
		if len(path.Features) == 0 {
			return nil, nil
		}
		return elements(e.context.Model.FeatureTypeSet(path.Features[len(path.Features)-1])), nil
	}
	return nil, e.unevaluable(expression, feature, value, nil)
}

// evaluateCollection applies a sequence function to what its source yields.
func (e *executor) evaluateCollection(
	expression queryplan.Expression,
	column string,
	row Value,
	tracker *propertyTracker,
) ([]Value, error) {
	source, ok := argumentValue(expression, "source")
	if !ok {
		return nil, e.invalidArgument(expression, "source", "")
	}
	values, err := e.evaluateColumnExpression(source, column, row, tracker)
	if err != nil {
		return nil, err
	}
	function := expression.Target()
	at := func(value Value) []Value { return []Value{valueAt(value, expression.Origin())} }
	switch function {
	case "size":
		return at(IntegerValue(int64(len(values)))), nil
	case "isEmpty":
		return at(BooleanValue(len(values) == 0)), nil
	case "notEmpty":
		return at(BooleanValue(len(values) > 0)), nil
	case "head":
		if len(values) == 0 {
			return nil, nil
		}
		return values[:1], nil
	case "last":
		if len(values) == 0 {
			return nil, nil
		}
		return values[len(values)-1:], nil
	case "distinct":
		return distinctValues(values), nil
	case "includes", "excludes", "including", "excluding":
		argument, ok := argumentValue(expression, "value")
		if !ok {
			return nil, e.invalidArgument(expression, "value", "")
		}
		operand, err := e.evaluateColumnExpression(argument, column, row, tracker)
		if err != nil {
			return nil, err
		}
		return e.applySetFunction(function, values, operand, at), nil
	}
	body, ok := argumentValue(expression, "body")
	if !ok || body.Operation() != queryplan.OperationLambda {
		return nil, e.invalidArgument(expression, "body", string(body.Operation()))
	}
	result, ok := argumentValue(body, "result")
	if !ok {
		return nil, e.invalidArgument(expression, "body", "")
	}
	_, binds := body.Literal()
	var selected []Value
	for _, value := range values {
		if !e.bindable(value, binds) {
			continue
		}
		release := e.bindVariable(body.Target(), value)
		outcome, err := e.evaluateColumnExpression(result, column, row, tracker)
		release()
		if err != nil {
			return nil, err
		}
		if function == "collect" {
			selected = append(selected, outcome...)
			continue
		}
		holds, err := e.predicateHolds(expression, column, row, result, outcome)
		if err != nil {
			return nil, err
		}
		switch function {
		case "select":
			if holds {
				selected = append(selected, value)
			}
		case "reject":
			if !holds {
				selected = append(selected, value)
			}
		case "exists":
			if holds {
				return at(BooleanValue(true)), nil
			}
		case "forAll":
			if !holds {
				return at(BooleanValue(false)), nil
			}
		}
	}
	switch function {
	case "exists":
		return at(BooleanValue(false)), nil
	case "forAll":
		return at(BooleanValue(true)), nil
	}
	return selected, nil
}

// bindable tells whether a body's parameter declared of the metaclass binds
// takes value: any value when none is declared, else an element of it; a
// connector end is a feature of its connection.
func (e *executor) bindable(value Value, binds string) bool {
	if binds == "" {
		return true
	}
	if _, _, ok := value.ConnectorEnd(); ok {
		return e.metaclassSpecializes("KerML::Core::Feature", binds)
	}
	sym, ok := value.Element()
	return ok && e.context.Model.MetaclassConforms(sym, binds)
}

// metaclassSpecializes tells whether the metaclass named meta is general
// itself or specializes it.
func (e *executor) metaclassSpecializes(meta, general string) bool {
	for _, target := range e.context.Index.LookupQualified(general) {
		for _, sym := range e.context.Index.LookupQualified(meta) {
			if symbols.SameElement(sym, target) || e.context.Model.Conforms(sym, target) {
				return true
			}
		}
	}
	return false
}

// distinctValues keeps the first of each run of equal values, in order.
func distinctValues(values []Value) []Value {
	var kept []Value
	for _, value := range values {
		duplicate := false
		for _, seen := range kept {
			if valuesEqual(seen, value) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			kept = append(kept, value)
		}
	}
	return kept
}

// applySetFunction is includes, excludes, including or excluding of operand
// over values: membership by value equality, appending, or removing.
func (e *executor) applySetFunction(function string, values, operand []Value, at func(Value) []Value) []Value {
	contains := func(value Value) bool {
		for _, candidate := range values {
			if valuesEqual(candidate, value) {
				return true
			}
		}
		return false
	}
	switch function {
	case "includes", "excludes":
		all := true
		for _, value := range operand {
			if !contains(value) {
				all = false
				break
			}
		}
		return at(BooleanValue(all == (function == "includes")))
	case "including":
		return append(append([]Value(nil), values...), operand...)
	}
	var kept []Value
	for _, value := range values {
		excluded := false
		for _, candidate := range operand {
			if valuesEqual(value, candidate) {
				excluded = true
				break
			}
		}
		if !excluded {
			kept = append(kept, value)
		}
	}
	return kept
}

// predicateHolds reads a body's outcome as the one Boolean a predicate yields.
func (e *executor) predicateHolds(
	expression queryplan.Expression, column string, row Value, result queryplan.Expression, outcome []Value,
) (bool, error) {
	operator := "->" + expression.Target()
	if len(outcome) != 1 {
		return false, e.columnError(ErrorColumnOperand, column, row, result.Origin(), operator, strconv.Itoa(len(outcome)))
	}
	holds, ok := outcome[0].Boolean()
	if !ok {
		return false, e.columnError(ErrorColumnOperandType, column, row, result.Origin(), operator, string(outcome[0].Kind()))
	}
	return holds, nil
}

// valuesEqual is KerML `==` on two values: the same element, object or
// connector end, or scalars of equal content, integers and reals by number.
func valuesEqual(left, right Value) bool {
	if l, ok := left.Element(); ok {
		r, ok := right.Element()
		return ok && symbols.SameElement(l, r)
	}
	if connector, position, ok := left.ConnectorEnd(); ok {
		other, otherPosition, ok := right.ConnectorEnd()
		return ok && position == otherPosition && symbols.SameElement(connector, other)
	}
	if l, _, ok := left.Object(); ok {
		r, _, ok := right.Object()
		return ok && l == r
	}
	if arithmeticKind(left.Kind()) && arithmeticKind(right.Kind()) {
		if left.Kind() == ValueInteger && right.Kind() == ValueInteger {
			return left.integer.Equal(right.integer)
		}
		return realOperand(left) == realOperand(right)
	}
	if left.Kind() != right.Kind() {
		return false
	}
	switch left.Kind() {
	case ValueString:
		return left.text == right.text
	case ValueBoolean:
		return left.boolean == right.boolean
	case ValueQuantity:
		return left.quantity != nil && right.quantity != nil &&
			left.quantity.Unit.Term.Commensurable(right.quantity.Unit.Term) &&
			left.quantity.BaseMagnitude() == right.quantity.BaseMagnitude()
	case ValueInfinity:
		return true
	}
	return false
}

// sequencesEqual is KerML `==` on two sequences: equal length, equal values
// position by position.
func sequencesEqual(left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !valuesEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

// evaluateLogicalOperator applies a comparison, Boolean connective or
// classification test: `==` and `!=` compare the operand sequences; the
// connectives take one Boolean each, and the conditional `and`, `or` and
// `implies` leave their second operand unevaluated once the first decides
// them; `istype` and `@` test the one value.
func (e *executor) evaluateLogicalOperator(
	expression queryplan.Expression,
	column string,
	row Value,
	tracker *propertyTracker,
	operator string,
) ([]Value, bool, error) {
	switch operator {
	case "==", "!=", "and", "&", "or", "|", "xor", "implies", "not", "istype", "@":
	default:
		return nil, false, nil
	}
	var operands []queryplan.Argument
	for _, operand := range expression.Arguments() {
		if operand.Name != "type" {
			operands = append(operands, operand)
		}
	}
	at := func(value bool) []Value { return []Value{valueAt(BooleanValue(value), expression.Origin())} }
	truth := func(i int) (bool, error) {
		values, err := e.evaluateColumnExpression(operands[i].Value, column, row, tracker)
		if err != nil {
			return false, err
		}
		if len(values) != 1 {
			return false, e.columnError(ErrorColumnOperand, column, row, operands[i].Value.Origin(), operator, strconv.Itoa(len(values)))
		}
		value, ok := values[0].Boolean()
		if !ok {
			return false, e.columnError(ErrorColumnOperandType, column, row, expression.Origin(), operator, string(values[0].Kind()))
		}
		return value, nil
	}
	switch operator {
	case "==", "!=":
		left, err := e.evaluateColumnExpression(operands[0].Value, column, row, tracker)
		if err != nil {
			return nil, true, err
		}
		right, err := e.evaluateColumnExpression(operands[1].Value, column, row, tracker)
		if err != nil {
			return nil, true, err
		}
		return at(sequencesEqual(left, right) == (operator == "==")), true, nil
	case "istype", "@":
		typeArgument, ok := argumentValue(expression, "type")
		if !ok {
			return nil, true, e.invalidArgument(expression, "type", "")
		}
		target, _ := typeArgument.Element()
		values, err := e.evaluateColumnExpression(operands[0].Value, column, row, tracker)
		if err != nil {
			return nil, true, err
		}
		if len(values) != 1 {
			return nil, true, e.columnError(ErrorColumnOperand, column, row, expression.Origin(), operator, strconv.Itoa(len(values)))
		}
		return at(e.valueIsA(values[0], typeTest{name: typeArgument.Target(), target: target})), true, nil
	}
	first, err := truth(0)
	if err != nil {
		return nil, true, err
	}
	switch operator {
	case "not":
		return at(!first), true, nil
	case "and":
		if !first {
			return at(false), true, nil
		}
	case "or":
		if first {
			return at(true), true, nil
		}
	case "implies":
		if !first {
			return at(true), true, nil
		}
	}
	second, err := truth(1)
	if err != nil {
		return nil, true, err
	}
	switch operator {
	case "and", "&":
		return at(first && second), true, nil
	case "or", "|":
		return at(first || second), true, nil
	case "xor":
		return at(first != second), true, nil
	}
	return at(!first || second), true, nil
}
