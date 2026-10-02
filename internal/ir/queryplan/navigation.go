package queryplan

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Cell expressions navigate from their row and compute over the collections
// they reach: `row.connectorEnd->select {in e; e.type.name == "USB"}->size()`.
// A navigation reads one feature of each element an operand yields, checked
// against the operand's metaclass when it is known; a collection function is
// one of the KerML sequence functions, with a body bound per element.

// collectionFunction describes one supported `->` function: the operation
// the plan names it by, whether it takes a body or a value argument, and what
// it yields.
type collectionFunction struct {
	operation string
	body      bool
	value     bool
	result    func(source, body columnType) columnType
	// multiplicity is the result's cardinality when fixed; unknown otherwise.
	multiplicity Multiplicity
}

var (
	oneValue     = Multiplicity{Lower: 1, Upper: 1, Known: true}
	atMostOne    = Multiplicity{Lower: 0, Upper: 1, Known: true}
	sourceTyped  = func(source, _ columnType) columnType { return source }
	bodyTyped    = func(_, body columnType) columnType { return body }
	integerTyped = func(_, _ columnType) columnType { return scalarType(semantics.PrimInteger) }
	booleanTyped = func(_, _ columnType) columnType { return scalarType(semantics.PrimBoolean) }
)

// collectionFunctions are the library functions a cell may apply with `->`,
// by the qualified name the function the cell names resolves to: the KerML
// sequence and control functions, and DocumentQueries::Distinct.
var collectionFunctions = map[string]collectionFunction{
	"SequenceFunctions::size":      {operation: "size", result: integerTyped, multiplicity: oneValue},
	"SequenceFunctions::isEmpty":   {operation: "isEmpty", result: booleanTyped, multiplicity: oneValue},
	"SequenceFunctions::notEmpty":  {operation: "notEmpty", result: booleanTyped, multiplicity: oneValue},
	"SequenceFunctions::head":      {operation: "head", result: sourceTyped, multiplicity: atMostOne},
	"SequenceFunctions::last":      {operation: "last", result: sourceTyped, multiplicity: atMostOne},
	"SequenceFunctions::includes":  {operation: "includes", value: true, result: booleanTyped, multiplicity: oneValue},
	"SequenceFunctions::excludes":  {operation: "excludes", value: true, result: booleanTyped, multiplicity: oneValue},
	"SequenceFunctions::including": {operation: "including", value: true, result: sourceTyped},
	"SequenceFunctions::excluding": {operation: "excluding", value: true, result: sourceTyped},
	"ControlFunctions::select":     {operation: "select", body: true, result: sourceTyped},
	"ControlFunctions::reject":     {operation: "reject", body: true, result: sourceTyped},
	"ControlFunctions::collect":    {operation: "collect", body: true, result: bodyTyped},
	"ControlFunctions::exists":     {operation: "exists", body: true, result: booleanTyped, multiplicity: oneValue},
	"ControlFunctions::forAll":     {operation: "forAll", body: true, result: booleanTyped, multiplicity: oneValue},
	"DocumentQueries::Distinct":    {operation: "distinct", result: sourceTyped},
}

// compileCell compiles a cell body's result with the compiler's cell scope
// set, so the query operations it invokes see its variables.
func (c *compiler) compileCell(query, owner *symbols.Symbol, node ast.Node, row *columnRow) (Expression, columnType, error) {
	saved := c.cell
	c.cell = row
	defer func() { c.cell = saved }()
	return c.compileColumnExpression(query, owner, row.column, node, row)
}

// compileCellOperand compiles, inside a cell, an argument of a query operation
// that reads the cell's variables — `source = row`, `row.connectorEnd` — or
// computes over them; ok is false for expressions the query compiler owns.
func (c *compiler) compileCellOperand(query, owner *symbols.Symbol, node ast.Node) (typedExpression, bool, error) {
	if c.cell == nil {
		return typedExpression{}, false, nil
	}
	switch expression := node.(type) {
	case *ast.FeatureReference:
		if c.cell.lookup(expression.Name) == nil {
			return typedExpression{}, false, nil
		}
	case *ast.FeatureChainExpr:
		head, _, ok := columnChainParts(expression)
		if ok && c.cell.lookup(head) == nil {
			return typedExpression{}, false, nil
		}
	case *ast.OperatorExpr:
	case *ast.InvocationExpr:
		if expression.Operand == nil {
			return typedExpression{}, false, nil
		}
	default:
		return typedExpression{}, false, nil
	}
	compiled, kind, err := c.compileColumnExpression(query, owner, c.cell.column, node, c.cell)
	if err != nil {
		return typedExpression{}, true, err
	}
	return typedExpression{
		expression:   compiled,
		types:        symbolSlice(kind.element),
		multiplicity: compiled.multiplicity,
	}, true, nil
}

// compileCellInvocation compiles a query operation a cell invokes over its
// variables, such as RelatedElements(source = row, …).
func (c *compiler) compileCellInvocation(
	query, owner *symbols.Symbol,
	column string,
	expression *ast.InvocationExpr,
	row *columnRow,
) (Expression, columnType, error) {
	root := row.root()
	saved := c.cell
	c.cell = row
	defer func() { c.cell = saved }()
	typed, err := c.compileInvocation(query, owner, root.params, expression, root.dependency)
	if err != nil {
		return Expression{}, columnType{}, err
	}
	kind := columnType{}
	if len(typed.types) == 1 {
		kind.element = typed.types[0]
		if prim := c.model.PrimTypeOf(kind.element); prim != semantics.PrimUnknown {
			kind = scalarType(prim)
		}
	}
	return typed.expression, kind, nil
}

// compileNavigationChain compiles a chain whose head is a variable, a
// collection, or any other expression yielding elements: each member reads a
// feature of the elements reached so far.
func (c *compiler) compileNavigationChain(
	query, owner *symbols.Symbol,
	column string,
	expression *ast.FeatureChainExpr,
	row *columnRow,
) (Expression, columnType, error) {
	if expression.Member == nil || len(expression.Member.Parts) != 1 {
		return Expression{}, columnType{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: column,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	operand, kind, err := c.compileColumnExpression(query, owner, column, expression.Operand, row)
	if err != nil {
		return Expression{}, columnType{}, err
	}
	return c.compileNavigation(query, owner, column, expression, operand, kind, expression.Member.Parts[0].Text)
}

// compileNavigation plans reading feature from the elements operand yields.
// An operand of known metaclass must declare the feature, which types the
// result; otherwise the feature is looked up on each element at execution.
func (c *compiler) compileNavigation(
	query, owner *symbols.Symbol,
	column string,
	node ast.Node,
	operand Expression,
	kind columnType,
	feature string,
) (Expression, columnType, error) {
	if kind.prim != semantics.PrimUnknown {
		return Expression{}, columnType{}, &Error{
			Kind:      ErrorUnknownColumnProperty,
			Query:     symbols.FQNOf(query),
			Target:    column,
			Parameter: feature,
			Actual:    kind.prim.String(),
			Origin:    symbols.NodeOrigin(owner.DocName, node),
		}
	}
	result := columnType{}
	declaring := ""
	if isMetaclassSymbol(kind.element) {
		target, ok := c.model.LookupMember(kind.element, feature)
		if !ok || !target.IsFeature() {
			return Expression{}, columnType{}, &Error{
				Kind:      ErrorUnknownColumnProperty,
				Query:     symbols.FQNOf(query),
				Target:    column,
				Parameter: feature,
				Actual:    symbols.FQNOf(kind.element),
				Origin:    symbols.NodeOrigin(owner.DocName, node),
			}
		}
		declaring = declaringTypeFQN(target)
		result = c.staticType(target)
		if feature == "documentation" {
			// Element::documentation reads as the prose of its bodies, as the
			// row property and Project's property do.
			result = scalarType(semantics.PrimString)
		}
	}
	multiplicity := Multiplicity{}
	if operand.multiplicity.Known && !operand.multiplicity.UpperInfinite && operand.multiplicity.Upper <= 1 &&
		declaring != "" {
		target, _ := c.model.LookupMember(kind.element, feature)
		multiplicity = c.featureMultiplicity(target)
		if operand.multiplicity.Lower == 0 {
			multiplicity.Lower = 0
		}
	}
	return Expression{
		operation:    OperationNavigate,
		target:       feature,
		value:        declaring,
		multiplicity: multiplicity,
		arguments:    []Argument{{Name: "source", Value: operand}},
		origin:       symbols.NodeOrigin(owner.DocName, node),
	}, result, nil
}

// compileCollection compiles `operand->function(…)`: a sequence function over
// what the operand yields, its body bound to each element in turn.
func (c *compiler) compileCollection(
	query, owner *symbols.Symbol,
	column string,
	expression *ast.InvocationExpr,
	row *columnRow,
) (Expression, columnType, error) {
	name := expression.Type.Text()
	fail := func(why string) error {
		return &Error{
			Kind:     ErrorCollectionFunction,
			Query:    symbols.FQNOf(query),
			Target:   column,
			Actual:   name,
			Expected: why,
			Origin:   symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	resolved, ok := c.resolver.ResolveQualified(owner.Scope, expression.Type)
	if !ok || resolved == nil {
		return Expression{}, columnType{}, fail("it names no function visible here")
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
		resolved = canonical
	}
	function, ok := collectionFunctions[symbols.FQNOf(resolved)]
	if !ok {
		return Expression{}, columnType{}, fail("not a sequence function a cell applies")
	}
	if len(expression.NamedArgs) > 0 {
		return Expression{}, columnType{}, fail("it takes no named arguments")
	}
	operand, kind, err := c.compileColumnExpression(query, owner, column, expression.Operand, row)
	if err != nil {
		return Expression{}, columnType{}, err
	}
	arguments := []Argument{{Name: "source", Value: operand}}
	bodyKind := columnType{}
	switch {
	case function.body:
		if len(expression.Args) != 1 {
			return Expression{}, columnType{}, fail("it takes one body")
		}
		body, ok := expression.Args[0].(*ast.BodyExpr)
		if !ok || len(body.Params) != 1 || len(body.Members) > 0 || body.Result == nil {
			return Expression{}, columnType{}, fail("its body binds one parameter to a result expression")
		}
		variable := &columnRow{name: body.Params[0].Name, typeSymbol: kind.element, outer: row, nested: true}
		// A parameter declared of a metaclass binds the elements of that
		// metaclass alone; the others are passed over, and the elements kept
		// are of it.
		binds := ""
		if body.Params[0].Type != nil {
			declared, ok := c.resolver.ResolveQualified(symbols.BodyExprScope(owner.Scope, body), body.Params[0].Type)
			if !ok || declared == nil {
				return Expression{}, columnType{}, &Error{
					Kind:      ErrorUnknownColumnProperty,
					Query:     symbols.FQNOf(query),
					Target:    column,
					Parameter: body.Params[0].Type.Text(),
					Origin:    symbols.NodeOrigin(owner.DocName, body.Params[0].Type),
				}
			}
			if canonical, ok := c.resolver.ResolveAliasTarget(declared); ok {
				declared = canonical
			}
			variable.typeSymbol = declared
			if isMetaclassSymbol(declared) {
				binds = symbols.FQNOf(declared)
				kind = columnType{element: declared}
			}
		}
		result, resultKind, err := c.compileColumnExpression(query, owner, column, body.Result, variable)
		if err != nil {
			return Expression{}, columnType{}, err
		}
		bodyKind = resultKind
		arguments = append(arguments, Argument{Name: "body", Value: Expression{
			operation: OperationLambda,
			target:    variable.name,
			value:     binds,
			arguments: []Argument{{Name: "result", Value: result}},
			origin:    symbols.NodeOrigin(owner.DocName, body),
		}})
	case function.value:
		if len(expression.Args) != 1 {
			return Expression{}, columnType{}, fail("it takes one argument")
		}
		value, _, err := c.compileColumnExpression(query, owner, column, expression.Args[0], row)
		if err != nil {
			return Expression{}, columnType{}, err
		}
		arguments = append(arguments, Argument{Name: "value", Value: value})
	default:
		if len(expression.Args) != 0 {
			return Expression{}, columnType{}, fail("it takes no arguments")
		}
	}
	return Expression{
		operation:    OperationCollection,
		target:       function.operation,
		multiplicity: function.multiplicity,
		arguments:    arguments,
		origin:       symbols.NodeOrigin(owner.DocName, expression),
	}, function.result(kind, bodyKind), nil
}

// compileLogicalOperator compiles the comparison, Boolean and classification
// operators a cell may apply: ==, !=, and, or, xor, implies, not, istype,
// hastype and @. Each yields one Boolean; a classification names its type.
func (c *compiler) compileLogicalOperator(
	query, owner *symbols.Symbol,
	column string,
	expression *ast.OperatorExpr,
	row *columnRow,
) (Expression, columnType, error) {
	invalid := &Error{
		Kind:   ErrorColumnOperator,
		Query:  symbols.FQNOf(query),
		Target: column,
		Actual: expression.Operator.String(),
		Origin: symbols.NodeOrigin(owner.DocName, expression),
	}
	arity := 2
	classification := false
	switch expression.Operator {
	case ast.OpNot:
		arity = 1
	case ast.OpIsType, ast.OpAt:
		arity, classification = 1, true
	}
	if len(expression.Operands) != arity || classification != (expression.TypeRef != nil) {
		return Expression{}, columnType{}, invalid
	}
	arguments := make([]Argument, 0, arity+1)
	kinds := make([]semantics.PrimType, 0, arity)
	for _, operand := range expression.Operands {
		compiled, kind, err := c.compileColumnExpression(query, owner, column, operand, row)
		if err != nil {
			return Expression{}, columnType{}, err
		}
		arguments = append(arguments, Argument{Value: compiled})
		kinds = append(kinds, kind.prim)
	}
	if classification {
		target, ok := c.resolver.ResolveQualified(owner.Scope, expression.TypeRef)
		if !ok || target == nil {
			return Expression{}, columnType{}, &Error{
				Kind:      ErrorUnknownColumnProperty,
				Query:     symbols.FQNOf(query),
				Target:    column,
				Parameter: expression.TypeRef.Text(),
				Origin:    symbols.NodeOrigin(owner.DocName, expression.TypeRef),
			}
		}
		if canonical, ok := c.resolver.ResolveAliasTarget(target); ok {
			target = canonical
		}
		arguments = append(arguments, Argument{Name: "type", Value: Expression{
			operation: OperationElement,
			target:    symbols.FQNOf(target),
			element:   target,
			origin:    symbols.NodeOrigin(owner.DocName, expression.TypeRef),
		}})
	}
	if err := c.validateLogicalOperands(query, owner, column, expression, kinds); err != nil {
		return Expression{}, columnType{}, err
	}
	return Expression{
		operation:    OperationColumnOperator,
		value:        expression.Operator.String(),
		multiplicity: oneValue,
		arguments:    arguments,
		origin:       symbols.NodeOrigin(owner.DocName, expression),
	}, scalarType(semantics.PrimBoolean), nil
}

// validateLogicalOperands rejects a Boolean connective over a statically
// non-Boolean operand; comparisons accept any kinds.
func (c *compiler) validateLogicalOperands(
	query, owner *symbols.Symbol,
	column string,
	expression *ast.OperatorExpr,
	kinds []semantics.PrimType,
) error {
	switch expression.Operator {
	case ast.OpEq, ast.OpNeq, ast.OpIsType, ast.OpAt:
		return nil
	}
	names := make([]string, 0, len(kinds))
	mismatch := false
	for _, kind := range kinds {
		names = append(names, primTypeName(kind))
		if kind != semantics.PrimUnknown && kind != semantics.PrimBoolean {
			mismatch = true
		}
	}
	if !mismatch {
		return nil
	}
	return &Error{
		Kind:      ErrorColumnType,
		Query:     symbols.FQNOf(query),
		Target:    column,
		Parameter: expression.Operator.String(),
		Actual:    strings.Join(names, " and "),
		Origin:    symbols.NodeOrigin(owner.DocName, expression),
	}
}

// isMetaclassSymbol reports whether a type is a reflective metaclass of the
// abstract syntax, which the stdlib declares under KerML and SysML.
func isMetaclassSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	fqn := symbols.FQNOf(sym)
	return strings.HasPrefix(fqn, "KerML::") || strings.HasPrefix(fqn, "SysML::")
}
