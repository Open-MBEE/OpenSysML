package queryexec

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Context provides the semantic workspace used by one execution.
type Context struct {
	Index    *symbols.Index
	Resolver *resolve.Resolver
	Model    *semantics.Model
}

// Options controls bounded query execution.
type Options struct {
	VisitBudget      int
	InvocationDepth  int
	InvocationBudget int
}

const (
	defaultVisitBudget      = 100_000
	defaultInvocationDepth  = 64
	defaultInvocationBudget = 10_000
)

type sequence struct {
	values  []Value
	columns []Column
	cells   [][]Cell
}

type visitBudget struct {
	remaining int
}

type executor struct {
	definition queryplan.Definition
	context    Context
	reader     *query.PropertyReader
	bindings   map[string]sequence
	program    map[string]queryplan.Definition
	budget     *visitBudget
	calls      *visitBudget
	related    *relationshipTables
	derived    *derivedValues
	depthLeft  int
	stack      []string
}

// Execute evaluates a compiled entry query into an immutable ordered row set.
func Execute(program *queryplan.Program, context Context, bindings Bindings, options Options) (*RowSet, error) {
	if program == nil || context.Index == nil || context.Resolver == nil || context.Model == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	definition, ok := entryDefinition(program)
	if !ok {
		return nil, &Error{Kind: ErrorInvalidContext, Query: program.Entry()}
	}
	budget := options.VisitBudget
	if budget == 0 {
		budget = defaultVisitBudget
	}
	depth := options.InvocationDepth
	if depth == 0 {
		depth = defaultInvocationDepth
	}
	calls := options.InvocationBudget
	if calls == 0 {
		calls = defaultInvocationBudget
	}
	if budget < 0 || depth < 0 || calls < 0 {
		return nil, &Error{Kind: ErrorInvalidContext, Query: definition.Name()}
	}
	definitions := program.Definitions()
	compiled := make(map[string]queryplan.Definition, len(definitions))
	for _, compiledDefinition := range definitions {
		compiled[compiledDefinition.Name()] = compiledDefinition
	}
	execution := &executor{
		definition: definition,
		context:    context,
		reader:     query.NewPropertyReader(context.Index, context.Resolver, context.Model),
		bindings:   make(map[string]sequence),
		program:    compiled,
		budget:     &visitBudget{remaining: budget},
		calls:      &visitBudget{remaining: calls},
		related:    newRelationshipTables(),
		derived:    &derivedValues{},
		depthLeft:  depth,
		stack:      []string{definition.Name()},
	}
	if err := execution.bind(bindings); err != nil {
		return nil, err
	}
	result, err := execution.evaluate(definition.Expression())
	if err != nil {
		return nil, err
	}
	if err := execution.validateResult(result); err != nil {
		return nil, err
	}
	rows := make([]Row, len(result.values))
	for i, value := range result.values {
		var cells []Cell
		if i < len(result.cells) {
			cells = cloneCells(result.cells[i])
		}
		rows[i] = Row{element: value, cells: cells}
	}
	return &RowSet{
		columns: append([]Column(nil), result.columns...),
		rows:    rows,
		origin:  definition.Origin(),
	}, nil
}

func entryDefinition(program *queryplan.Program) (queryplan.Definition, bool) {
	for _, definition := range program.Definitions() {
		if definition.Name() == program.Entry() {
			return definition, true
		}
	}
	return queryplan.Definition{}, false
}

func (e *executor) bind(bindings Bindings) error {
	parameters := e.definition.Parameters()
	known := make(map[string]queryplan.Parameter, len(parameters))
	for _, parameter := range parameters {
		known[parameter.Name] = parameter
	}
	for name := range bindings {
		if _, ok := known[name]; !ok {
			return &Error{
				Kind:      ErrorUnknownBinding,
				Query:     e.definition.Name(),
				Parameter: name,
				Origin:    e.definition.Origin(),
			}
		}
	}
	defaulted := make([]queryplan.Parameter, 0)
	for _, parameter := range parameters {
		values, present := bindings[parameter.Name]
		if !present {
			if parameter.HasDefault {
				defaulted = append(defaulted, parameter)
				continue
			}
			if parameter.Multiplicity.Known && parameter.Multiplicity.Lower == 0 {
				e.bindings[parameter.Name] = sequence{}
				continue
			}
			return &Error{
				Kind:      ErrorMissingBinding,
				Query:     e.definition.Name(),
				Parameter: parameter.Name,
				Origin:    parameter.Origin,
			}
		}
		if err := e.bindValues(parameter, values); err != nil {
			return err
		}
	}
	// Defaults are evaluated once per execution, in parameter order, after the explicit bindings.
	for _, parameter := range defaulted {
		value, err := e.evaluate(parameter.Default)
		if err != nil {
			return err
		}
		if err := e.bindValues(parameter, value.values); err != nil {
			return err
		}
	}
	return nil
}

// bindValues checks one parameter's values against its effective type and
// multiplicity, whether they were supplied by the caller or by a default.
func (e *executor) bindValues(parameter queryplan.Parameter, values []Value) error {
	if !withinMultiplicity(int64(len(values)), parameter.Multiplicity) {
		return &Error{
			Kind:      ErrorBindingMultiplicity,
			Query:     e.definition.Name(),
			Parameter: parameter.Name,
			Expected:  multiplicityString(parameter.Multiplicity),
			Actual:    strconv.Itoa(len(values)),
			Origin:    parameter.Origin,
		}
	}
	for _, value := range values {
		if !e.valueConforms(value, parameter.Type) {
			return &Error{
				Kind:      ErrorBindingType,
				Query:     e.definition.Name(),
				Parameter: parameter.Name,
				Expected:  parameter.Type,
				Actual:    string(value.Kind()),
				Origin:    parameter.Origin,
			}
		}
	}
	e.bindings[parameter.Name] = sequence{values: append([]Value(nil), values...)}
	return nil
}

func withinMultiplicity(count int64, multiplicity queryplan.Multiplicity) bool {
	if !multiplicity.Known {
		return true
	}
	if count < multiplicity.Lower {
		return false
	}
	return multiplicity.UpperInfinite || count <= multiplicity.Upper
}

func multiplicityString(multiplicity queryplan.Multiplicity) string {
	if !multiplicity.Known {
		return "unknown"
	}
	upper := strconv.FormatInt(multiplicity.Upper, 10)
	if multiplicity.UpperInfinite {
		upper = "*"
	}
	return fmt.Sprintf("%d..%s", multiplicity.Lower, upper)
}

func (e *executor) valueConforms(value Value, expected string) bool {
	switch value.Kind() {
	case ValueElement:
		sym, ok := value.Element()
		if !ok {
			return false
		}
		targets := e.context.Index.LookupQualified(expected)
		for _, target := range targets {
			if e.context.Model.IsDataType(target) {
				return e.context.Model.LiteralConforms(sym, target)
			}
			if symbols.SameElement(sym, target) || e.context.Model.Conforms(sym, target) {
				return true
			}
		}
		return expected == "Element" || expected == "KerML::Root::Element"
	case ValueQuantity:
		quantity, ok := value.Quantity()
		if !ok {
			return false
		}
		for _, target := range e.context.Index.LookupQualified(expected) {
			if c := e.context.Model.QuantityConforms(quantity.Unit.Term, target); c.Known && c.Holds {
				return true
			}
		}
		return false
	default:
		actual, ok := scalarValueType(value)
		if !ok {
			return false
		}
		for _, target := range e.context.Index.LookupQualified(expected) {
			expectedType := e.context.Model.PrimTypeOf(target)
			if expectedType != semantics.PrimUnknown && semantics.PrimConforms(actual, expectedType) {
				return true
			}
		}
		return false
	}
}

func scalarValueType(value Value) (semantics.PrimType, bool) {
	switch value.Kind() {
	case ValueBoolean:
		return semantics.PrimBoolean, true
	case ValueString:
		return semantics.PrimString, true
	case ValueInteger:
		return semantics.PrimInteger, true
	case ValueReal:
		return semantics.PrimReal, true
	default:
		return semantics.PrimUnknown, false
	}
}

func (e *executor) evaluate(expression queryplan.Expression) (sequence, error) {
	switch expression.Operation() {
	case queryplan.OperationParameter:
		value, ok := e.bindings[expression.Target()]
		if !ok {
			return sequence{}, e.errorAt(ErrorMissingBinding, expression)
		}
		return cloneSequence(value), nil
	case queryplan.OperationElement:
		return e.evaluateElement(expression)
	case queryplan.OperationLiteral:
		return e.evaluateLiteral(expression)
	case queryplan.OperationSequence:
		return e.evaluateSequence(expression)
	case queryplan.OperationOwnedElements:
		return e.evaluateOwned(expression)
	case queryplan.OperationDescendants:
		return e.evaluateDescendants(expression)
	case queryplan.OperationAncestors:
		return e.evaluateAncestors(expression)
	case queryplan.OperationWhereType:
		return e.evaluateWhereType(expression)
	case queryplan.OperationWhereMetadata:
		return e.evaluateWhereMetadata(expression)
	case queryplan.OperationWhereName:
		return e.evaluateWhereName(expression)
	case queryplan.OperationWhereFeature:
		return e.evaluateWhereFeature(expression)
	case queryplan.OperationOrderBy:
		return e.evaluateOrderBy(expression)
	case queryplan.OperationProject:
		return e.evaluateProject(expression)
	case queryplan.OperationInvoke:
		return e.evaluateInvoke(expression)
	case queryplan.OperationRelatedElements:
		return e.evaluateRelated(expression)
	default:
		return sequence{}, &Error{
			Kind:      ErrorUnsupportedOperation,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Origin:    expression.Origin(),
		}
	}
}

func (e *executor) evaluateInvoke(expression queryplan.Expression) (sequence, error) {
	target := expression.Target()
	definition, ok := e.program[target]
	if !ok {
		return sequence{}, &Error{
			Kind:      ErrorUnknownInvocation,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Target:    target,
			Origin:    expression.Origin(),
		}
	}
	if slices.Contains(e.stack, target) {
		return sequence{}, &Error{
			Kind:      ErrorInvocationCycle,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Target:    target,
			Path:      append(append([]string(nil), e.stack...), target),
			Origin:    expression.Origin(),
		}
	}
	if e.depthLeft <= 0 {
		return sequence{}, &Error{
			Kind:      ErrorInvocationDepth,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Target:    target,
			Origin:    expression.Origin(),
		}
	}
	if e.calls.remaining <= 0 {
		return sequence{}, &Error{
			Kind:      ErrorInvocationBudget,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Target:    target,
			Origin:    expression.Origin(),
		}
	}
	e.calls.remaining--
	bindings := make(Bindings, len(expression.Arguments()))
	for _, argument := range expression.Arguments() {
		value, err := e.evaluate(argument.Value)
		if err != nil {
			return sequence{}, err
		}
		// A projected argument binds its row elements; columns and cells do not cross a binding.
		bindings[argument.Name] = value.values
	}
	callee := &executor{
		definition: definition,
		context:    e.context,
		reader:     e.reader,
		bindings:   make(map[string]sequence),
		program:    e.program,
		budget:     e.budget,
		calls:      e.calls,
		related:    e.related,
		derived:    e.derived,
		depthLeft:  e.depthLeft - 1,
		stack:      append(append([]string(nil), e.stack...), target),
	}
	if err := callee.bind(bindings); err != nil {
		return sequence{}, err
	}
	result, err := callee.evaluate(definition.Expression())
	if err != nil {
		return sequence{}, err
	}
	if err := callee.validateResult(result); err != nil {
		return sequence{}, err
	}
	return result, nil
}

func (e *executor) evaluateElement(expression queryplan.Expression) (sequence, error) {
	element, ok := expression.Element()
	if !ok {
		return sequence{}, e.errorAt(ErrorMissingElement, expression)
	}
	return sequence{values: []Value{valueAt(ElementValue(element), expression.Origin())}}, nil
}

func (e *executor) evaluateLiteral(expression queryplan.Expression) (sequence, error) {
	kind, raw := expression.Literal()
	origin := expression.Origin()
	var value Value
	switch kind {
	case queryplan.LiteralString:
		text, err := strconv.Unquote(raw)
		if err != nil {
			return sequence{}, e.invalidArgument(expression, "", raw)
		}
		value = StringValue(text)
	case queryplan.LiteralInteger:
		integer, err := strconv.ParseInt(strings.ReplaceAll(raw, "_", ""), 10, 64)
		if err != nil {
			return sequence{}, e.invalidArgument(expression, "", raw)
		}
		value = IntegerValue(integer)
	case queryplan.LiteralReal:
		realVal, err := strconv.ParseFloat(strings.ReplaceAll(raw, "_", ""), 64)
		if err != nil || math.IsInf(realVal, 0) || math.IsNaN(realVal) {
			return sequence{}, e.invalidArgument(expression, "", raw)
		}
		value = RealValue(realVal)
	case queryplan.LiteralBoolean:
		boolean, err := strconv.ParseBool(raw)
		if err != nil {
			return sequence{}, e.invalidArgument(expression, "", raw)
		}
		value = BooleanValue(boolean)
	case queryplan.LiteralInfinity:
		value = Value{kind: ValueInfinity}
	case queryplan.LiteralNull:
		return sequence{}, nil
	default:
		return sequence{}, e.invalidArgument(expression, "", raw)
	}
	return sequence{values: []Value{valueAt(value, origin)}}, nil
}

func (e *executor) evaluateSequence(expression queryplan.Expression) (sequence, error) {
	var result sequence
	for _, argument := range expression.Arguments() {
		value, err := e.evaluate(argument.Value)
		if err != nil {
			return sequence{}, err
		}
		if len(value.columns) > 0 {
			return sequence{}, e.invalidArgument(expression, argument.Name, "projected row set")
		}
		result.values = append(result.values, value.values...)
	}
	return result, nil
}

func hasArgument(expression queryplan.Expression, name string) bool {
	_, ok := argumentValue(expression, name)
	return ok
}

func argumentValue(expression queryplan.Expression, name string) (queryplan.Expression, bool) {
	for _, argument := range expression.Arguments() {
		if argument.Name == name {
			return argument.Value, true
		}
	}
	return queryplan.Expression{}, false
}

func (e *executor) argument(expression queryplan.Expression, name string) (sequence, error) {
	for _, argument := range expression.Arguments() {
		if argument.Name == name {
			return e.evaluate(argument.Value)
		}
	}
	return sequence{}, e.invalidArgument(expression, name, "missing")
}

func (e *executor) elementArgument(expression queryplan.Expression, name string) (sequence, error) {
	value, err := e.argument(expression, name)
	if err != nil {
		return sequence{}, err
	}
	for _, item := range value.values {
		if _, ok := item.Element(); !ok {
			return sequence{}, e.invalidArgument(expression, name, string(item.Kind()))
		}
	}
	return value, nil
}

func (e *executor) stringArgument(expression queryplan.Expression, name string) (string, error) {
	value, err := e.argument(expression, name)
	if err != nil {
		return "", err
	}
	if len(value.values) != 1 {
		return "", e.invalidArgument(expression, name, strconv.Itoa(len(value.values)))
	}
	text, ok := value.values[0].String()
	if !ok {
		return "", e.invalidArgument(expression, name, string(value.values[0].Kind()))
	}
	return text, nil
}

func (e *executor) stringsArgument(expression queryplan.Expression, name string) ([]string, error) {
	value, err := e.argument(expression, name)
	if err != nil {
		return nil, err
	}
	texts := make([]string, len(value.values))
	for i, item := range value.values {
		text, ok := item.String()
		if !ok {
			return nil, e.invalidArgument(expression, name, string(item.Kind()))
		}
		texts[i] = text
	}
	return texts, nil
}

func (e *executor) integerArgument(expression queryplan.Expression, name string) (int64, error) {
	value, err := e.argument(expression, name)
	if err != nil {
		return 0, err
	}
	if len(value.values) != 1 {
		return 0, e.invalidArgument(expression, name, strconv.Itoa(len(value.values)))
	}
	integer, ok := value.values[0].Integer()
	if !ok || integer < 0 {
		return 0, e.invalidArgument(expression, name, string(value.values[0].Kind()))
	}
	return integer, nil
}

func (e *executor) invalidArgument(expression queryplan.Expression, parameter, actual string) error {
	return &Error{
		Kind:      ErrorInvalidArgument,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Parameter: parameter,
		Actual:    actual,
		Origin:    expression.Origin(),
	}
}

func (e *executor) errorAt(kind ErrorKind, expression queryplan.Expression) error {
	return &Error{
		Kind:      kind,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Origin:    expression.Origin(),
	}
}

func cloneSequence(input sequence) sequence {
	result := sequence{
		values:  append([]Value(nil), input.values...),
		columns: append([]Column(nil), input.columns...),
		cells:   make([][]Cell, len(input.cells)),
	}
	for i := range input.cells {
		result.cells[i] = cloneCells(input.cells[i])
	}
	return result
}

func cloneCells(input []Cell) []Cell {
	result := make([]Cell, len(input))
	for i, cell := range input {
		result[i] = Cell{values: cell.Values(), origin: cell.origin}
	}
	return result
}

func (e *executor) validateResult(result sequence) error {
	for _, value := range result.values {
		if value.Kind() != ValueElement || !e.valueConforms(value, e.definition.Result().Type) {
			actual := string(value.Kind())
			if sym, ok := value.Element(); ok {
				actual = symbols.FQNOf(sym)
			}
			return &Error{
				Kind:     ErrorResultType,
				Query:    e.definition.Name(),
				Expected: e.definition.Result().Type,
				Actual:   actual,
				Origin:   e.definition.Result().Origin,
			}
		}
	}
	if !withinMultiplicity(int64(len(result.values)), e.definition.Result().Multiplicity) {
		return &Error{
			Kind:     ErrorResultMultiplicity,
			Query:    e.definition.Name(),
			Expected: multiplicityString(e.definition.Result().Multiplicity),
			Actual:   strconv.Itoa(len(result.values)),
			Origin:   e.definition.Result().Origin,
		}
	}
	return nil
}
