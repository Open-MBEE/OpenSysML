package queryexec

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// rowKey identifies a row for traversal: an element by identity, an object by
// its session identity.
type rowKey struct {
	element symbols.ElementKey
	object  int64
}

func keyOfRow(row Value) rowKey {
	if inst, _, ok := row.Object(); ok {
		return rowKey{object: inst.ID}
	}
	sym, _ := row.Element()
	return rowKey{element: symbols.KeyOf(sym)}
}

// ownedRows returns the rows a row owns: the members of an element's scope, or
// the objects an object's features hold.
func (e *executor) ownedRows(expression queryplan.Expression, row Value) ([]Value, error) {
	if _, _, ok := row.Object(); ok {
		return e.heldObjects(expression, row)
	}
	sym, _ := row.Element()
	if sym.Scope == nil {
		return nil, nil
	}
	members := sym.Scope.AllMembers()
	out := make([]Value, 0, len(members))
	for _, member := range members {
		out = append(out, ElementValue(member))
	}
	return out, nil
}

// ownerRow returns the row owning a row: the owner of an element's scope, or the
// object holding an object.
func (e *executor) ownerRow(row Value) (Value, bool) {
	if _, _, ok := row.Object(); ok {
		return e.objectOwner(row)
	}
	sym, _ := row.Element()
	if sym.OwnerScope == nil {
		return Value{}, false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return Value{}, false
	}
	return ElementValue(owner), true
}

func (e *executor) evaluateOwned(expression queryplan.Expression) (sequence, error) {
	source, err := e.ownershipArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	seen := make(map[rowKey]struct{})
	for _, value := range source.values {
		owned, err := e.ownedRows(expression, value)
		if err != nil {
			return sequence{}, err
		}
		for _, member := range owned {
			key := keyOfRow(member)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			seen[key] = struct{}{}
			result.values = append(result.values, member)
		}
	}
	return result, nil
}

func (e *executor) evaluateDescendants(expression queryplan.Expression) (sequence, error) {
	source, err := e.ownershipArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		row   Value
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[rowKey]struct{})
	for _, value := range source.values {
		seen[keyOfRow(value)] = struct{}{}
		queue = append(queue, pending{row: value})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth {
			continue
		}
		owned, err := e.ownedRows(expression, next.row)
		if err != nil {
			return sequence{}, err
		}
		for _, member := range owned {
			key := keyOfRow(member)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			seen[key] = struct{}{}
			result.values = append(result.values, member)
			queue = append(queue, pending{row: member, depth: next.depth + 1})
		}
	}
	return result, nil
}

func (e *executor) evaluateAncestors(expression queryplan.Expression) (sequence, error) {
	source, err := e.ownershipArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		row   Value
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[rowKey]struct{})
	for _, value := range source.values {
		seen[keyOfRow(value)] = struct{}{}
		queue = append(queue, pending{row: value})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth {
			continue
		}
		owner, ok := e.ownerRow(next.row)
		if !ok {
			continue
		}
		key := keyOfRow(owner)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		if !e.consumeVisit() {
			return sequence{}, e.budgetError(expression)
		}
		seen[key] = struct{}{}
		result.values = append(result.values, owner)
		queue = append(queue, pending{row: owner, depth: next.depth + 1})
	}
	return result, nil
}

func (e *executor) evaluateWhereType(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	typeName, err := e.stringArgument(expression, "type")
	if err != nil {
		return sequence{}, err
	}
	target := e.resolveClassification(typeName)
	classification := typeName
	if target != nil {
		classification = symbols.FQNOf(target)
	}
	result := filtered(source)
	for i, value := range source.values {
		if _, _, isObject := value.Object(); isObject {
			if e.objectIsA(value, typeName, target) {
				appendSelected(&result, source, i)
			}
			continue
		}
		sym := value.Declaration()
		if sym == nil {
			continue
		}
		matches := query.MetamodelTypeNameOf(sym) == typeName
		if target != nil {
			matches = matches ||
				e.context.Model.MetaclassConforms(sym, classification) ||
				symbols.SameElement(sym, target) ||
				e.context.Model.Conforms(sym, target)
		}
		if matches {
			appendSelected(&result, source, i)
		}
	}
	if target == nil && len(result.values) == 0 && !query.IsMetamodelTypeName(typeName) {
		return sequence{}, &Error{
			Kind:      ErrorUnknownClassification,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    typeName,
			Origin:    expression.Origin(),
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereMetadata(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	name, err := e.stringArgument(expression, "metadata")
	if err != nil {
		return sequence{}, err
	}
	target := e.resolveClassification(name)
	if target == nil {
		return sequence{}, &Error{
			Kind:      ErrorUnknownClassification,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    name,
			Origin:    expression.Origin(),
		}
	}
	result := filtered(source)
	for i, value := range source.values {
		sym := value.Declaration()
		for _, annotation := range e.context.Model.AnnotationFactsOf(sym) {
			types := e.context.Index.LookupQualified(annotation.TypeFQN)
			matches := false
			for _, actual := range types {
				if symbols.SameElement(actual, target) || e.context.Model.Conforms(actual, target) {
					matches = true
					break
				}
			}
			if matches {
				appendSelected(&result, source, i)
				break
			}
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereName(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	operator, err := e.stringArgument(expression, "operator")
	if err != nil {
		return sequence{}, err
	}
	expected, err := e.stringArgument(expression, "value")
	if err != nil {
		return sequence{}, err
	}
	if compareErr := validateTextComparison(operator, expected); compareErr != nil {
		if compareErr != errComparison {
			return sequence{}, e.invalidArgument(expression, "value", expected)
		}
		return sequence{}, e.operatorError(expression, operator)
	}
	result := filtered(source)
	for i, value := range source.values {
		names, _, nameErr := e.propertyValues(value, query.PropertyName)
		if nameErr != nil {
			return sequence{}, nameErr
		}
		if len(names) == 0 {
			continue
		}
		name, _ := names[0].String()
		match, compareErr := compareText(name, operator, expected)
		if compareErr != nil {
			if compareErr != errComparison {
				return sequence{}, e.invalidArgument(expression, "value", expected)
			}
			return sequence{}, e.operatorError(expression, operator)
		}
		if match {
			appendSelected(&result, source, i)
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereFeature(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	property, err := e.stringArgument(expression, "feature")
	if err != nil {
		return sequence{}, err
	}
	operator, err := e.stringArgument(expression, "operator")
	if err != nil {
		return sequence{}, err
	}
	expected, err := e.stringArgument(expression, "value")
	if err != nil {
		return sequence{}, err
	}
	if compareErr := validateFeatureComparison(operator, expected); compareErr != nil {
		if compareErr != errComparison {
			return sequence{}, e.invalidArgument(expression, "value", expected)
		}
		return sequence{}, e.operatorError(expression, operator)
	}
	if len(source.values) == 0 {
		return source, nil
	}
	result := filtered(source)
	known := false
	columnIndex := projectedColumn(source, property)
	for i := range source.values {
		values, present, valueErr := e.featureValues(expression, source, i, property, columnIndex)
		if valueErr != nil {
			return sequence{}, valueErr
		}
		known = known || present
		for _, actual := range values {
			match, compareErr := compareValue(actual, operator, expected)
			if compareErr != nil {
				if compareErr != errComparison {
					return sequence{}, e.invalidArgument(expression, "value", expected)
				}
				return sequence{}, e.operatorError(expression, operator)
			}
			if match {
				appendSelected(&result, source, i)
				break
			}
		}
	}
	if !known {
		return sequence{}, e.unknownProperty(expression, property)
	}
	return result, nil
}

func (e *executor) evaluateOrderBy(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	property, err := e.stringArgument(expression, "property")
	if err != nil {
		return sequence{}, err
	}
	direction, err := e.stringArgument(expression, "direction")
	if err != nil {
		return sequence{}, err
	}
	missing, err := e.stringArgument(expression, "missing")
	if err != nil {
		return sequence{}, err
	}
	multiple, err := e.stringArgument(expression, "multiple")
	if err != nil {
		return sequence{}, err
	}
	if direction != "ascending" && direction != "descending" {
		return sequence{}, e.operatorError(expression, direction)
	}
	if missing != "first" && missing != "last" && missing != "error" {
		return sequence{}, e.operatorError(expression, missing)
	}
	if multiple != "first" && multiple != "last" && multiple != "error" {
		return sequence{}, e.operatorError(expression, multiple)
	}
	if len(source.values) == 0 {
		return source, nil
	}
	type sortable struct {
		value Value
		cells []Cell
		key   Value
		set   bool
	}
	columnIndex := projectedColumn(source, property)
	items := make([]sortable, len(source.values))
	known := false
	var firstKey Value
	for i, value := range source.values {
		values, present, valueErr := e.featureValues(expression, source, i, property, columnIndex)
		if valueErr != nil {
			return sequence{}, valueErr
		}
		known = known || present
		items[i].value = value
		if i < len(source.cells) {
			items[i].cells = cloneCells(source.cells[i])
		}
		switch len(values) {
		case 0:
			if missing == "error" {
				return sequence{}, e.featureError(expression, property, value)
			}
		case 1:
			items[i].key = values[0]
			items[i].set = true
		default:
			if multiple == "error" {
				return sequence{}, e.featureError(expression, property, value)
			}
			index := 0
			if multiple == "last" {
				index = len(values) - 1
			}
			items[i].key = values[index]
			items[i].set = true
		}
		if items[i].set {
			if firstKey.Kind() == "" {
				firstKey = items[i].key
			} else if !e.orderedKeysCompatible(firstKey, items[i].key) {
				return sequence{}, e.invalidOrder(expression, property, firstKey, items[i].key)
			}
		}
	}
	if !known {
		return sequence{}, e.unknownProperty(expression, property)
	}
	var sortErr error
	var sortKeys [2]Value
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.set != right.set {
			if missing == "first" {
				return !left.set
			}
			return left.set
		}
		if !left.set {
			return false
		}
		comparison, err := e.compareOrdered(left.key, right.key)
		if err != nil {
			sortErr = err
			sortKeys = [2]Value{left.key, right.key}
			return false
		}
		if direction == "descending" {
			comparison = -comparison
		}
		return comparison < 0
	})
	if sortErr != nil {
		return sequence{}, e.invalidOrder(expression, property, sortKeys[0], sortKeys[1])
	}
	result := sequence{columns: append([]Column(nil), source.columns...)}
	for _, item := range items {
		result.values = append(result.values, item.value)
		result.cells = append(result.cells, item.cells)
	}
	return result, nil
}

// projectedColumn is the index of the projected column named property, or -1;
// a feature naming a projected column reads its cells, so computed and
// relationship-derived columns are filterable and orderable by name.
func projectedColumn(source sequence, property string) int {
	for i, column := range source.columns {
		if column.name == property {
			return i
		}
	}
	return -1
}

// featureValues reads one row's feature: its projected cell when columnIndex
// names one, otherwise the property of the row itself.
func (e *executor) featureValues(
	expression queryplan.Expression,
	source sequence,
	row int,
	property string,
	columnIndex int,
) ([]Value, bool, error) {
	if columnIndex >= 0 && row < len(source.cells) {
		return source.cells[row][columnIndex].Values(), true, nil
	}
	value := source.values[row]
	values, present, err := e.propertyValues(value, property)
	if err != nil {
		return nil, false, e.unevaluable(expression, property, value, err)
	}
	return values, present, nil
}

func (e *executor) evaluateProject(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var properties []string
	if hasArgument(expression, "properties") {
		properties, err = e.stringsArgument(expression, "properties")
		if err != nil {
			return sequence{}, err
		}
	}
	var computed []computedColumn
	if columnsValue, ok := argumentValue(expression, "columns"); ok {
		computed, err = e.computedColumns(expression, columnsValue)
		if err != nil {
			return sequence{}, err
		}
	}
	total := len(properties) + len(computed)
	if total == 0 {
		return sequence{}, e.invalidArgument(expression, "properties", "empty")
	}
	seen := make(map[string]bool, total)
	for _, property := range properties {
		seen[property] = true
	}
	for _, column := range computed {
		if seen[column.name] {
			return sequence{}, e.invalidArgument(expression, "columns", column.name)
		}
		seen[column.name] = true
	}
	result := sequence{
		values:  append([]Value(nil), source.values...),
		columns: make([]Column, total),
		cells:   make([][]Cell, len(source.values)),
	}
	known := make([]bool, len(properties))
	for i, property := range properties {
		result.columns[i] = Column{name: property, origin: expression.Origin()}
	}
	for i, column := range computed {
		result.columns[len(properties)+i] = Column{name: column.name, origin: column.origin.Origin()}
	}
	tracker := newPropertyTracker()
	for row, value := range source.values {
		result.cells[row] = make([]Cell, total)
		for column, property := range properties {
			values, present, valueErr := e.propertyValues(value, property)
			if valueErr != nil {
				return sequence{}, e.unevaluable(expression, property, value, valueErr)
			}
			known[column] = known[column] || present
			result.cells[row][column] = Cell{
				values: values,
				origin: value.Origin(),
			}
		}
		for i, column := range computed {
			values, cellErr := e.evaluateColumnCell(column, value, tracker)
			if cellErr != nil {
				return sequence{}, cellErr
			}
			result.cells[row][len(properties)+i] = Cell{
				values: values,
				origin: value.Origin(),
			}
		}
	}
	if len(source.values) > 0 {
		for i, property := range properties {
			if !known[i] {
				return sequence{}, e.unknownProperty(expression, property)
			}
		}
		if property, missing := tracker.missing(); missing {
			return sequence{}, e.unknownProperty(expression, property)
		}
	}
	return result, nil
}

// propertyValues reads a property of a row: of the session for an object row,
// of the check, state or trace record for a verdict, state or event row, of
// the model for an element row.
func (e *executor) propertyValues(row Value, property string) ([]Value, bool, error) {
	if _, _, isObject := row.Object(); isObject {
		return e.objectPropertyValues(row, property)
	}
	if _, isVerdict := row.Verdict(); isVerdict {
		return e.verdictPropertyValues(row, property)
	}
	if _, isState := row.State(); isState {
		return e.statePropertyValues(row, property)
	}
	if _, isEvent := row.Event(); isEvent {
		return e.eventPropertyValues(row, property)
	}
	sym, _ := row.Element()
	if isQueryableProperty(property) {
		values, present := e.reader.Values(sym, property)
		if !present {
			return nil, true, nil
		}
		result := make([]Value, 0, len(values))
		for _, value := range values {
			result = append(result, typedPropertyValue(property, value, sym))
		}
		return result, true, nil
	}
	return e.declaredFeatureValues(sym, property)
}

// declaredFeatureValues reads a declared (non-metadata) feature of a row.
func (e *executor) declaredFeatureValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
	values, present := e.context.Model.DeclaredFeatureValues(sym, property)
	if !present {
		return nil, false, nil
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := filterValue(value, sym)
		if !ok {
			if value.Kind == symbols.FilterValueEmpty {
				continue
			}
			if value.Kind == symbols.FilterValueUnknown {
				derived, err := e.derivedFeatureValues(sym, property)
				return derived, true, err
			}
			return nil, true, e.featureError(queryplan.Expression{}, property, ElementValue(sym))
		}
		result = append(result, converted)
	}
	return result, true, nil
}

func isQueryableProperty(property string) bool {
	switch property {
	case query.PropertyID,
		query.PropertyType,
		query.PropertyName,
		query.PropertyDeclaredName,
		query.PropertyShortName,
		query.PropertyDeclaredShortName,
		query.PropertyDocumentation,
		query.PropertyQualifiedName,
		query.PropertyOwner,
		query.PropertyElementType,
		query.PropertyIsAbstract,
		query.PropertyMultiplicityLower,
		query.PropertyMultiplicityUpper:
		return true
	default:
		return false
	}
}

func typedPropertyValue(property, value string, sym *symbols.Symbol) Value {
	var result Value
	switch property {
	case query.PropertyIsAbstract:
		boolean, _ := strconv.ParseBool(value)
		result = BooleanValue(boolean)
	case query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper:
		if value == "*" {
			result = Value{kind: ValueInfinity}
		} else {
			integer, _ := strconv.ParseInt(value, 10, 64)
			result = IntegerValue(integer)
		}
	default:
		result = StringValue(value)
	}
	return valueAt(result, ElementValue(sym).Origin())
}

func filterValue(value symbols.FilterValue, sym *symbols.Symbol) (Value, bool) {
	var result Value
	switch value.Kind {
	case symbols.FilterValueBool:
		result = BooleanValue(value.Bool)
	case symbols.FilterValueInt:
		result = IntegerValue(value.Int)
	case symbols.FilterValueReal:
		result = RealValue(value.Real)
	case symbols.FilterValueString:
		result = StringValue(value.Str)
	case symbols.FilterValueRef:
		result = StringValue(value.RefFQN)
	case symbols.FilterValueQuantity:
		quantity, ok := semantics.QuantityOf(value)
		if !ok {
			return Value{}, false
		}
		result = QuantityValue(*quantity)
	default:
		return Value{}, false
	}
	return valueAt(result, ElementValue(sym).Origin()), true
}

// The hyphenated spellings of the text operators, accepted beside the camelCase ones.
const (
	opStartsWith = "starts-with"
	opEndsWith   = "ends-with"
)

func compareText(actual, operator, expected string) (bool, error) {
	switch operator {
	case "=", "==":
		return actual == expected, nil
	case "!=", "<>":
		return actual != expected, nil
	case "contains":
		return strings.Contains(actual, expected), nil
	case "startsWith", opStartsWith:
		return strings.HasPrefix(actual, expected), nil
	case "endsWith", opEndsWith:
		return strings.HasSuffix(actual, expected), nil
	case "matches":
		expression, err := regexp.Compile(expected)
		if err != nil {
			return false, err
		}
		return expression.MatchString(actual), nil
	default:
		return false, errComparison
	}
}

func validateTextComparison(operator, expected string) error {
	switch operator {
	case "=", "==", "!=", "<>", "contains", "startsWith", opStartsWith, "endsWith", opEndsWith:
		return nil
	case "matches":
		_, err := regexp.Compile(expected)
		return err
	default:
		return errComparison
	}
}

func validateFeatureComparison(operator, expected string) error {
	switch operator {
	case "=", "==", "!=", "<>", "contains", "startsWith", opStartsWith, "endsWith", opEndsWith:
		return nil
	case "matches":
		_, err := regexp.Compile(expected)
		return err
	case "<", "<=", ">", ">=":
		_, err := parseNumericValue(expected)
		return err
	default:
		return errComparison
	}
}

var errComparison = &comparisonError{}

type comparisonError struct{}

func (*comparisonError) Error() string { return "unsupported comparison" }

func compareValue(actual Value, operator, expected string) (bool, error) {
	switch actual.Kind() {
	case ValueString:
		value, _ := actual.String()
		return compareText(value, operator, expected)
	case ValueBoolean:
		value, _ := actual.Boolean()
		want, err := strconv.ParseBool(expected)
		if err != nil {
			return false, err
		}
		switch operator {
		case "=", "==":
			return value == want, nil
		case "!=", "<>":
			return value != want, nil
		default:
			return false, errComparison
		}
	case ValueInteger, ValueReal, ValueInfinity:
		want, err := parseNumericValue(expected)
		if err != nil {
			return false, err
		}
		return compareOrdinal(compareNumeric(actual, want), operator)
	case ValueQuantity:
		// A bare number compares against the magnitude in the quantity's own unit.
		magnitude, _ := actual.Magnitude()
		return compareValue(magnitude, operator, expected)
	case ValueElement:
		// An element compares as the qualified name a cell prints it by.
		sym, _ := actual.Element()
		return compareText(symbols.FQNOf(sym), operator, expected)
	default:
		return false, errComparison
	}
}

func parseNumericValue(text string) (Value, error) {
	if text == "*" {
		return Value{kind: ValueInfinity}, nil
	}
	text = strings.ReplaceAll(text, "_", "")
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return IntegerValue(integer), nil
	}
	realVal, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(realVal) || math.IsInf(realVal, 0) {
		return Value{}, strconv.ErrSyntax
	}
	return RealValue(realVal), nil
}

func compareOrdinal(comparison int, operator string) (bool, error) {
	switch operator {
	case "=", "==":
		return comparison == 0, nil
	case "!=", "<>":
		return comparison != 0, nil
	case "<":
		return comparison < 0, nil
	case "<=":
		return comparison <= 0, nil
	case ">":
		return comparison > 0, nil
	case ">=":
		return comparison >= 0, nil
	default:
		return false, errComparison
	}
}

// compareOrdered orders two keys orderedKeysCompatible admitted; quantities
// compare on the left key's reference, a point on a scale through its anchor.
func (e *executor) compareOrdered(left, right Value) (int, error) {
	if numericKind(left.Kind()) && numericKind(right.Kind()) {
		return compareNumeric(left, right), nil
	}
	if left.Kind() != right.Kind() {
		return strings.Compare(string(left.Kind()), string(right.Kind())), nil
	}
	switch left.Kind() {
	case ValueQuantity:
		l, _ := left.Quantity()
		r, _ := right.Quantity()
		return e.derived.get(e.context).CompareMagnitudes(l, r)
	case ValueString:
		l, _ := left.String()
		r, _ := right.String()
		return strings.Compare(l, r), nil
	case ValueElement:
		l, _ := left.Element()
		r, _ := right.Element()
		return strings.Compare(symbols.FQNOf(l), symbols.FQNOf(r)), nil
	case ValueBoolean:
		l, _ := left.Boolean()
		r, _ := right.Boolean()
		if !l && r {
			return -1, nil
		}
		if l && !r {
			return 1, nil
		}
	}
	return 0, nil
}

func compareNumeric(left, right Value) int {
	if left.Kind() == ValueInfinity {
		if right.Kind() == ValueInfinity {
			return 0
		}
		return 1
	}
	if right.Kind() == ValueInfinity {
		return -1
	}
	if left.Kind() == ValueInteger && right.Kind() == ValueReal {
		l, _ := left.Integer()
		r, _ := right.Real()
		return compareIntReal(l, r)
	}
	if left.Kind() == ValueReal && right.Kind() == ValueInteger {
		l, _ := left.Real()
		r, _ := right.Integer()
		return -compareIntReal(r, l)
	}
	switch left.Kind() {
	case ValueInteger:
		l, _ := left.Integer()
		r, _ := right.Integer()
		return compareInt(l, r)
	case ValueReal:
		l, _ := left.Real()
		r, _ := right.Real()
		return compareFloat(l, r)
	}
	return 0
}

func compareIntReal(integer int64, realVal float64) int {
	left := new(big.Rat).SetInt64(integer)
	right := new(big.Rat).SetFloat64(realVal)
	return left.Cmp(right)
}

func compareInt(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareFloat(left, right float64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

// orderedKeysCompatible reports whether two sort keys are comparable: values of
// one kind, numbers of any kind, or quantities the runtime can order.
func (e *executor) orderedKeysCompatible(left, right Value) bool {
	if left.Kind() == ValueQuantity && right.Kind() == ValueQuantity {
		l, _ := left.Quantity()
		r, _ := right.Quantity()
		_, err := e.derived.get(e.context).CompareMagnitudes(l, r)
		return err == nil
	}
	if left.Kind() == right.Kind() {
		return true
	}
	return numericKind(left.Kind()) && numericKind(right.Kind())
}

func numericKind(kind ValueKind) bool {
	return kind == ValueInteger || kind == ValueReal || kind == ValueInfinity
}

func (e *executor) resolveClassification(name string) *symbols.Symbol {
	if matches := e.context.Index.LookupQualified(name); len(matches) == 1 {
		return matches[0]
	}
	var match *symbols.Symbol
	for _, fqn := range e.context.Index.FQNs() {
		if fqn != name && !strings.HasSuffix(fqn, "::"+name) {
			continue
		}
		candidates := e.context.Index.LookupQualified(fqn)
		for _, candidate := range candidates {
			if match != nil && !symbols.SameElement(match, candidate) {
				return nil
			}
			match = candidate
		}
	}
	return match
}

// filtered starts a filter's result, keeping the source's projected columns
// even when no row is selected.
func filtered(source sequence) sequence {
	return sequence{columns: append([]Column(nil), source.columns...)}
}

func appendSelected(result *sequence, source sequence, index int) {
	result.values = append(result.values, source.values[index])
	if index < len(source.cells) {
		result.cells = append(result.cells, cloneCells(source.cells[index]))
	}
}

func (e *executor) consumeVisit() bool {
	if e.budget.remaining <= 0 {
		return false
	}
	e.budget.remaining--
	return true
}

func (e *executor) budgetError(expression queryplan.Expression) error {
	return &Error{
		Kind:      ErrorVisitBudget,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Origin:    expression.Origin(),
	}
}

func (e *executor) operatorError(expression queryplan.Expression, operator string) error {
	return &Error{
		Kind:      ErrorInvalidOperator,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Actual:    operator,
		Origin:    expression.Origin(),
	}
}

func (e *executor) unknownProperty(expression queryplan.Expression, property string) error {
	return &Error{
		Kind:      ErrorUnknownProperty,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Origin:    expression.Origin(),
	}
}

// invalidOrder reports sort keys that cannot be ordered; two quantity keys name
// their incommensurable units.
func (e *executor) invalidOrder(expression queryplan.Expression, property string, first, other Value) error {
	err := &Error{
		Kind:      ErrorInvalidOrder,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Origin:    expression.Origin(),
	}
	if l, ok := first.Quantity(); ok {
		if r, ok := other.Quantity(); ok {
			err.Expected = l.Unit.String()
			err.Actual = r.Unit.String()
		}
	}
	return err
}

func (e *executor) featureError(expression queryplan.Expression, property string, row Value) error {
	return e.unevaluable(expression, property, row, nil)
}

// unevaluable is featureError carrying the evaluator's reason, when one is known.
func (e *executor) unevaluable(expression queryplan.Expression, property string, row Value, cause error) error {
	var inner *Error
	if errors.As(cause, &inner) && inner.Kind == ErrorUnevaluableFeature {
		cause = inner.Cause
	}
	return &Error{
		Kind:      ErrorUnevaluableFeature,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Target:    rowTarget(row),
		Origin:    expression.Origin(),
		Cause:     cause,
	}
}

// rowTarget names a row in an error: an element by qualified name, an object
// by the label the session reaches it by, a verdict, state or event by its label.
func rowTarget(row Value) string {
	if _, label, ok := row.Object(); ok {
		return label
	}
	if verdict, ok := row.Verdict(); ok {
		return verdict.Label()
	}
	if state, ok := row.State(); ok {
		return state.Label()
	}
	if event, ok := row.Event(); ok {
		return event.Label()
	}
	sym, _ := row.Element()
	return symbols.FQNOf(sym)
}
