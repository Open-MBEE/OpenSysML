package queryexec

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (e *executor) evaluateOwned(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		if sym.Scope == nil {
			continue
		}
		for _, member := range sym.Scope.AllMembers() {
			key := symbols.KeyOf(member)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			seen[key] = struct{}{}
			result.values = append(result.values, ElementValue(member))
		}
	}
	return result, nil
}

func (e *executor) evaluateDescendants(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		key := symbols.KeyOf(sym)
		seen[key] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth || next.sym.Scope == nil {
			continue
		}
		for _, member := range next.sym.Scope.AllMembers() {
			key := symbols.KeyOf(member)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			seen[key] = struct{}{}
			result.values = append(result.values, ElementValue(member))
			queue = append(queue, pending{sym: member, depth: next.depth + 1})
		}
	}
	return result, nil
}

func (e *executor) evaluateAncestors(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		seen[symbols.KeyOf(sym)] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth || next.sym.OwnerScope == nil {
			continue
		}
		owner := next.sym.OwnerScope.Owner()
		if owner == nil {
			continue
		}
		key := symbols.KeyOf(owner)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		if !e.consumeVisit() {
			return sequence{}, e.budgetError(expression)
		}
		seen[key] = struct{}{}
		result.values = append(result.values, ElementValue(owner))
		queue = append(queue, pending{sym: owner, depth: next.depth + 1})
	}
	return result, nil
}

func (e *executor) evaluateWhereType(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
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
		sym, _ := value.Element()
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
	source, err := e.elementArgument(expression, "source")
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
		sym, _ := value.Element()
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
	source, err := e.elementArgument(expression, "source")
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
		sym, _ := value.Element()
		names, _ := e.reader.Values(sym, query.PropertyName)
		if len(names) == 0 {
			continue
		}
		match, compareErr := compareText(names[0], operator, expected)
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
	source, err := e.elementArgument(expression, "source")
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
	for i, value := range source.values {
		sym, _ := value.Element()
		values, present, valueErr := e.propertyValues(sym, property)
		if valueErr != nil {
			return sequence{}, e.unevaluable(expression, property, sym, valueErr)
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
	source, err := e.elementArgument(expression, "source")
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
	// A property matching a projected column orders by its cells, so a
	// computed column is orderable by name.
	columnIndex := -1
	for i, column := range source.columns {
		if column.name == property {
			columnIndex = i
			break
		}
	}
	items := make([]sortable, len(source.values))
	known := false
	var firstKey Value
	for i, value := range source.values {
		sym, _ := value.Element()
		var values []Value
		var present bool
		if columnIndex >= 0 && i < len(source.cells) {
			values = source.cells[i][columnIndex].Values()
			present = true
		} else {
			var valueErr error
			values, present, valueErr = e.propertyValues(sym, property)
			if valueErr != nil {
				return sequence{}, e.unevaluable(expression, property, sym, valueErr)
			}
		}
		known = known || present
		items[i].value = value
		if i < len(source.cells) {
			items[i].cells = cloneCells(source.cells[i])
		}
		switch len(values) {
		case 0:
			if missing == "error" {
				return sequence{}, e.featureError(expression, property, sym)
			}
		case 1:
			items[i].key = values[0]
			items[i].set = true
		default:
			if multiple == "error" {
				return sequence{}, e.featureError(expression, property, sym)
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

func (e *executor) evaluateProject(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
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
		sym, _ := value.Element()
		result.cells[row] = make([]Cell, total)
		for column, property := range properties {
			values, present, valueErr := e.propertyValues(sym, property)
			if valueErr != nil {
				return sequence{}, e.unevaluable(expression, property, sym, valueErr)
			}
			known[column] = known[column] || present
			result.cells[row][column] = Cell{
				values: values,
				origin: value.Origin(),
			}
		}
		for i, column := range computed {
			values, cellErr := e.evaluateColumnCell(column, sym, tracker)
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

func (e *executor) propertyValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
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
			return nil, true, e.featureError(queryplan.Expression{}, property, sym)
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

func (e *executor) featureError(expression queryplan.Expression, property string, sym *symbols.Symbol) error {
	return e.unevaluable(expression, property, sym, nil)
}

// unevaluable is featureError carrying the evaluator's reason, when one is known.
func (e *executor) unevaluable(expression queryplan.Expression, property string, sym *symbols.Symbol, cause error) error {
	var inner *Error
	if errors.As(cause, &inner) && inner.Kind == ErrorUnevaluableFeature {
		cause = inner.Cause
	}
	return &Error{
		Kind:      ErrorUnevaluableFeature,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Target:    symbols.FQNOf(sym),
		Origin:    expression.Origin(),
		Cause:     cause,
	}
}
