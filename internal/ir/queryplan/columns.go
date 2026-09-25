package queryplan

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	columnFQN        = "DocumentQueries::Column"
	columnSpecFQN    = "DocumentQueries::ColumnSpec"
	relatedColumnFQN = "DocumentQueries::RelatedColumn"
)

// compileColumns compiles Project's columns argument: a sequence of
// Column(name, expression) and RelatedColumn(...) invocations into a
// planned column sequence.
func (c *compiler) compileColumns(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	node ast.Node,
	dependency func(string),
) (Expression, error) {
	var elements []ast.Node
	switch value := node.(type) {
	case *ast.SequenceExpr:
		elements = value.Elements
	case *ast.NullExpr:
	default:
		elements = []ast.Node{node}
	}
	args := make([]Argument, 0, len(elements))
	for _, element := range elements {
		column, err := c.compileColumn(query, owner, params, element, dependency)
		if err != nil {
			return Expression{}, err
		}
		args = append(args, Argument{Value: column})
	}
	return Expression{
		operation: OperationSequence,
		arguments: args,
		origin:    symbols.NodeOrigin(owner.DocName, node),
	}, nil
}

func (c *compiler) compileColumn(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	node ast.Node,
	dependency func(string),
) (Expression, error) {
	invalid := &Error{
		Kind:   ErrorInvalidColumn,
		Query:  symbols.FQNOf(query),
		Origin: symbols.NodeOrigin(owner.DocName, node),
	}
	invocation, ok := node.(*ast.InvocationExpr)
	if !ok || invocation.Operand != nil {
		return Expression{}, invalid
	}
	selection := c.model.SelectCall(owner.Scope, invocation, semantics.PerformsBehavior)
	if selection.Ambiguous {
		return Expression{}, invalid
	}
	switch symbols.FQNOf(selection.Called()) {
	case relatedColumnFQN:
		return c.compileRelatedColumn(query, owner, params, selection.Called(), invocation, dependency)
	case columnFQN:
	default:
		return Expression{}, invalid
	}
	nameNode, expressionNode, err := c.columnArguments(query, owner, invocation)
	if err != nil {
		return Expression{}, err
	}
	name, err := c.columnName(query, owner, nameNode)
	if err != nil {
		return Expression{}, err
	}
	if expressionNode == nil {
		return Expression{}, &Error{
			Kind:      ErrorMissingArgument,
			Query:     symbols.FQNOf(query),
			Target:    columnFQN,
			Parameter: "expression",
			Origin:    symbols.NodeOrigin(owner.DocName, node),
		}
	}
	expression, _, err := c.compileColumnExpression(query, owner, name, expressionNode)
	if err != nil {
		return Expression{}, err
	}
	arguments := []Argument{{Name: "expression", Named: true, Value: expression}}
	return Expression{
		operation: OperationColumn,
		target:    name,
		arguments: arguments,
		origin:    symbols.NodeOrigin(owner.DocName, node),
	}, nil
}

// columnName reads a column's name, which must be a string literal.
func (c *compiler) columnName(query, owner *symbols.Symbol, nameNode ast.Node) (string, error) {
	literal, ok := nameNode.(*ast.LiteralString)
	if ok {
		_, err := strconv.Unquote(literal.Value)
		ok = err == nil
	}
	if !ok {
		return "", &Error{
			Kind:   ErrorColumnName,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, nameNode),
		}
	}
	name, _ := strconv.Unquote(literal.Value)
	return name, nil
}

// columnArguments normalizes Column's positional or named arguments into the
// name expression and the optional column expression.
func (c *compiler) columnArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	invocation *ast.InvocationExpr,
) (ast.Node, ast.Node, error) {
	if len(invocation.Args) > 0 {
		if len(invocation.NamedArgs) > 0 || len(invocation.Args) > 2 {
			return nil, nil, &Error{
				Kind:   ErrorArgumentCount,
				Query:  symbols.FQNOf(query),
				Target: columnFQN,
				Origin: symbols.NodeOrigin(owner.DocName, invocation),
			}
		}
		var expression ast.Node
		if len(invocation.Args) == 2 {
			expression = invocation.Args[1]
		}
		return invocation.Args[0], expression, nil
	}
	var name, expression ast.Node
	for _, arg := range invocation.NamedArgs {
		argName := arg.Name.Text()
		var slot *ast.Node
		switch argName {
		case "name":
			slot = &name
		case "expression":
			slot = &expression
		default:
			return nil, nil, &Error{
				Kind:      ErrorUnknownArgument,
				Query:     symbols.FQNOf(query),
				Target:    columnFQN,
				Parameter: argName,
				Origin:    symbols.NodeOrigin(owner.DocName, arg.Value),
			}
		}
		if *slot != nil {
			return nil, nil, &Error{
				Kind:      ErrorDuplicateArgument,
				Query:     symbols.FQNOf(query),
				Target:    columnFQN,
				Parameter: argName,
				Origin:    symbols.NodeOrigin(owner.DocName, arg.Value),
			}
		}
		*slot = arg.Value
	}
	if name == nil {
		return nil, nil, &Error{
			Kind:      ErrorMissingArgument,
			Query:     symbols.FQNOf(query),
			Target:    columnFQN,
			Parameter: "name",
			Origin:    symbols.NodeOrigin(owner.DocName, invocation),
		}
	}
	return name, expression, nil
}

// compileColumnExpression compiles a per-row calc expression into the plan's
// closed row-property, literal, parameter and operator operations, returning
// the statically known scalar type where one is derivable.
func (c *compiler) compileColumnExpression(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	column string,
	node ast.Node,
) (Expression, semantics.PrimType, error) {
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.compileColumnReference(query, owner, column, expression)
	case *ast.LiteralString:
		return c.literalExpression(owner, node, LiteralString, expression.Value, "").expression,
			semantics.PrimString, nil
	case *ast.LiteralInteger:
		return c.literalExpression(owner, node, LiteralInteger, expression.Value, "").expression,
			semantics.PrimInteger, nil
	case *ast.LiteralReal:
		return c.literalExpression(owner, node, LiteralReal, expression.Value, "").expression,
			semantics.PrimReal, nil
	case *ast.LiteralBool:
		return c.literalExpression(
				owner,
				node,
				LiteralBoolean,
				strconv.FormatBool(expression.Value),
				"",
			).expression,
			semantics.PrimBoolean, nil
	case *ast.NullExpr:
		return c.literalExpression(owner, node, LiteralNull, "null", "").expression,
			semantics.PrimUnknown, nil
	case *ast.OperatorExpr:
		return c.compileColumnOperator(query, owner, column, expression)
	case *ast.FeatureChainExpr:
		return c.compileColumnChain(query, owner, column, expression)
	default:
		return Expression{}, semantics.PrimUnknown, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: column,
			Origin: symbols.NodeOrigin(owner.DocName, node),
		}
	}
}

// compileColumnReference resolves a feature reference to a query parameter or
// to a declared feature read as a row property by its name.
func (c *compiler) compileColumnReference(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	column string,
	expression *ast.FeatureReference,
) (Expression, semantics.PrimType, error) {
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Name)
	if !ok {
		return Expression{}, semantics.PrimUnknown, &Error{
			Kind:      ErrorUnknownColumnProperty,
			Query:     symbols.FQNOf(query),
			Target:    column,
			Parameter: expression.Name.Text(),
			Origin:    symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	for _, param := range c.model.BehaviorParametersOf(query) {
		if !param.IsResult && c.parameterIncludes(param.Symbol, target) {
			return Expression{
				operation:    OperationParameter,
				target:       param.Symbol.Name,
				multiplicity: c.parameterMultiplicity(param.Symbol),
				origin:       symbols.NodeOrigin(owner.DocName, expression),
			}, c.staticPrimType(param.Symbol), nil
		}
	}
	return Expression{
		operation:    OperationRowProperty,
		target:       target.Name,
		value:        declaringTypeFQN(target),
		multiplicity: c.featureMultiplicity(target),
		origin:       symbols.NodeOrigin(owner.DocName, expression),
	}, c.staticPrimType(target), nil
}

// compileColumnChain compiles a feature chain — `stat.runs`, `'Monte
// Carlo'.runs` — into a member-path read: each segment names a member nested
// in the element the chain's head reaches, walked at execution. A head
// resolving in the owner's scope names a feature the row must conform to, as
// row-property does; an unresolved head — the common case, a member the row
// element itself declares — is row-relative. Its multiplicity is unknown: the
// feature the last segment names is found per row, so a member without a
// value is an empty cell rather than a typed failure.
func (c *compiler) compileColumnChain(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	column string,
	expression *ast.FeatureChainExpr,
) (Expression, semantics.PrimType, error) {
	unsupported := func() error {
		return &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: column,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	head, members, ok := columnChainParts(expression)
	if !ok {
		return Expression{}, semantics.PrimUnknown, unsupported()
	}
	var segments []string
	declaring := ""
	if target, resolved := c.resolver.ResolveQualified(owner.Scope, head); resolved && target != nil {
		for _, param := range c.model.BehaviorParametersOf(query) {
			if !param.IsResult && c.parameterIncludes(param.Symbol, target) {
				return Expression{}, semantics.PrimUnknown, unsupported()
			}
		}
		segments = append(segments, target.Name)
		declaring = declaringTypeFQN(target)
	} else {
		for _, part := range head.Parts {
			segments = append(segments, part.Text)
		}
	}
	for _, member := range members {
		for _, part := range member.Parts {
			segments = append(segments, part.Text)
		}
	}
	return Expression{
		operation:    OperationRowMember,
		target:       source.MemberPathOf(segments),
		value:        declaring,
		multiplicity: Multiplicity{},
		origin:       symbols.NodeOrigin(owner.DocName, expression),
	}, semantics.PrimUnknown, nil
}

// columnChainParts flattens a feature chain into its head name and the ordered
// member names the chain then walks; the head must be a feature reference or
// a bare qualified name.
func columnChainParts(expression *ast.FeatureChainExpr) (*ast.QualifiedName, []*ast.QualifiedName, bool) {
	var members []*ast.QualifiedName
	node := ast.Node(expression)
	for {
		chain, ok := node.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		if chain.Member == nil || len(chain.Member.Parts) == 0 {
			return nil, nil, false
		}
		members = append(members, chain.Member)
		node = chain.Operand
	}
	head := ast.AsQualifiedName(node)
	if head == nil || len(head.Parts) == 0 {
		return nil, nil, false
	}
	for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
		members[i], members[j] = members[j], members[i]
	}
	return head, members, true
}

// declaringTypeFQN names the type declaring a feature, so execution reads it
// only from rows conforming to that type; empty for non-type owners.
func declaringTypeFQN(target *symbols.Symbol) string {
	scope := target.OwnerScope
	if scope == nil {
		return ""
	}
	owner := scope.Owner()
	if owner == nil || owner == target {
		return ""
	}
	switch owner.Kind {
	case symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolUnknown:
		return ""
	}
	return symbols.FQNOf(owner)
}

func (c *compiler) staticPrimType(sym *symbols.Symbol) semantics.PrimType {
	typeSymbol := c.parameterTypeSymbol(sym)
	if typeSymbol == nil {
		return semantics.PrimUnknown
	}
	return c.model.PrimTypeOf(typeSymbol)
}

func (c *compiler) compileColumnOperator(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	column string,
	expression *ast.OperatorExpr,
) (Expression, semantics.PrimType, error) {
	arity := 0
	switch expression.Operator {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpNullCoalesce:
		arity = 2
	case ast.OpNeg, ast.OpPos:
		arity = 1
	default:
		return Expression{}, semantics.PrimUnknown, &Error{
			Kind:   ErrorColumnOperator,
			Query:  symbols.FQNOf(query),
			Target: column,
			Actual: expression.Operator.String(),
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	if len(expression.Operands) != arity {
		return Expression{}, semantics.PrimUnknown, &Error{
			Kind:   ErrorColumnOperator,
			Query:  symbols.FQNOf(query),
			Target: column,
			Actual: expression.Operator.String(),
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	arguments := make([]Argument, 0, arity)
	kinds := make([]semantics.PrimType, 0, arity)
	for _, operand := range expression.Operands {
		compiled, kind, err := c.compileColumnExpression(query, owner, column, operand)
		if err != nil {
			return Expression{}, semantics.PrimUnknown, err
		}
		arguments = append(arguments, Argument{Value: compiled})
		kinds = append(kinds, kind)
	}
	result, err := c.validateColumnOperator(query, owner, column, expression, kinds)
	if err != nil {
		return Expression{}, semantics.PrimUnknown, err
	}
	return Expression{
		operation: OperationColumnOperator,
		value:     expression.Operator.String(),
		arguments: arguments,
		origin:    symbols.NodeOrigin(owner.DocName, expression),
	}, result, nil
}

// validateColumnOperator rejects statically detectable operand type
// mismatches and derives the operator's result type where it is known.
func (c *compiler) validateColumnOperator(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	column string,
	expression *ast.OperatorExpr,
	kinds []semantics.PrimType,
) (semantics.PrimType, error) {
	mismatch := func() error {
		names := make([]string, len(kinds))
		for i, kind := range kinds {
			names[i] = primTypeName(kind)
		}
		actual := names[0]
		if len(names) == 2 {
			actual = names[0] + " and " + names[1]
		}
		return &Error{
			Kind:      ErrorColumnType,
			Query:     symbols.FQNOf(query),
			Target:    column,
			Parameter: expression.Operator.String(),
			Actual:    actual,
			Origin:    symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	switch expression.Operator {
	case ast.OpNullCoalesce:
		if kinds[0] == kinds[1] {
			return kinds[0], nil
		}
		return semantics.PrimUnknown, nil
	case ast.OpNeg, ast.OpPos:
		if kinds[0] == semantics.PrimUnknown || staticNumeric(kinds[0]) {
			return kinds[0], nil
		}
		return semantics.PrimUnknown, mismatch()
	case ast.OpAdd:
		if kinds[0] == semantics.PrimString && kinds[1] == semantics.PrimString {
			return semantics.PrimString, nil
		}
		if kinds[0] == semantics.PrimString || kinds[1] == semantics.PrimString {
			if kinds[0] == semantics.PrimUnknown || kinds[1] == semantics.PrimUnknown {
				return semantics.PrimUnknown, nil
			}
			return semantics.PrimUnknown, mismatch()
		}
	}
	for _, kind := range kinds {
		if kind != semantics.PrimUnknown && !staticNumeric(kind) {
			return semantics.PrimUnknown, mismatch()
		}
	}
	for _, kind := range kinds {
		if kind == semantics.PrimUnknown {
			return semantics.PrimUnknown, nil
		}
	}
	if kinds[0] == semantics.PrimInteger && (len(kinds) == 1 || kinds[1] == semantics.PrimInteger) {
		return semantics.PrimInteger, nil
	}
	return semantics.PrimReal, nil
}

func staticNumeric(kind semantics.PrimType) bool {
	switch kind {
	case semantics.PrimInteger, semantics.PrimNatural, semantics.PrimReal,
		semantics.PrimRational, semantics.PrimNumber:
		return true
	default:
		return false
	}
}

func primTypeName(kind semantics.PrimType) string {
	if kind == semantics.PrimUnknown {
		return "unknown"
	}
	return kind.String()
}

// validateProject requires a nonempty projection and rejects duplicate
// column names across literal properties and computed columns.
func (c *compiler) validateProject(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	expression *ast.InvocationExpr,
	args []Argument,
) error {
	var properties, columns *Argument
	for i := range args {
		switch args[i].Name {
		case "properties":
			properties = &args[i]
		case "columns":
			columns = &args[i]
		}
	}
	if staticallyEmpty(properties) && staticallyEmpty(columns) {
		return &Error{
			Kind:   ErrorEmptyProjection,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	seen := make(map[string]bool)
	if properties != nil {
		for _, name := range staticStrings(properties.Value) {
			seen[name] = true
		}
	}
	if columns == nil {
		return nil
	}
	for _, column := range columns.Value.arguments {
		name := column.Value.target
		if seen[name] {
			return &Error{
				Kind:      ErrorDuplicateColumn,
				Query:     symbols.FQNOf(query),
				Parameter: name,
				Origin:    column.Value.origin,
			}
		}
		seen[name] = true
	}
	return nil
}

// staticallyEmpty reports whether a projection argument is provably empty:
// absent, an explicit null, or an empty sequence.
func staticallyEmpty(argument *Argument) bool {
	if argument == nil {
		return true
	}
	switch argument.Value.operation {
	case OperationLiteral:
		return argument.Value.literal == LiteralNull
	case OperationSequence:
		return len(argument.Value.arguments) == 0
	}
	return false
}

// staticStrings collects the literal strings of a planned properties value;
// parameter-driven elements contribute nothing.
func staticStrings(value Expression) []string {
	if value.operation == OperationSequence {
		var out []string
		for _, element := range value.arguments {
			out = append(out, staticStrings(element.Value)...)
		}
		return out
	}
	if text, ok := staticString(value); ok {
		return []string{text}
	}
	return nil
}

// staticString reads a planned string literal; false for any other value.
func staticString(value Expression) (string, bool) {
	if value.operation == OperationLiteral && value.literal == LiteralString {
		if text, err := strconv.Unquote(value.value); err == nil {
			return text, true
		}
	}
	return "", false
}

// relatedColumnNameNode finds the name argument of a RelatedColumn invocation,
// positional or named; nil when it is absent.
func relatedColumnNameNode(invocation *ast.InvocationExpr) ast.Node {
	if len(invocation.Args) > 0 {
		return invocation.Args[0]
	}
	for _, arg := range invocation.NamedArgs {
		if arg.Name.Text() == "name" {
			return arg.Value
		}
	}
	return nil
}

// compileRelatedColumn compiles RelatedColumn(name, relationshipKind,
// direction, maxDepth, aggregate) into a planned related column named by its
// target, whose arguments are type-checked against the library signature.
func (c *compiler) compileRelatedColumn(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	target *symbols.Symbol,
	invocation *ast.InvocationExpr,
	dependency func(string),
) (Expression, error) {
	name := ""
	if nameNode := relatedColumnNameNode(invocation); nameNode != nil {
		literal, err := c.columnName(query, owner, nameNode)
		if err != nil {
			return Expression{}, err
		}
		name = literal
	}
	targetParams, _, err := c.signature(target, ignoreDependency)
	if err != nil {
		return Expression{}, err
	}
	args, err := c.compileBuiltinArguments(
		query,
		owner,
		params,
		invocation,
		relatedColumnFQN,
		targetParams,
		dependency,
	)
	if err != nil {
		return Expression{}, err
	}
	arguments := make([]Argument, 0, len(args))
	for _, arg := range args {
		if arg.Name == "name" {
			continue
		}
		if arg.Name == "aggregate" {
			if aggregate, literal := staticString(arg.Value); literal && !RelatedAggregateSupported(aggregate) {
				return Expression{}, &Error{
					Kind:   ErrorColumnAggregate,
					Query:  symbols.FQNOf(query),
					Target: name,
					Actual: aggregate,
					Origin: arg.Value.origin,
				}
			}
		}
		arguments = append(arguments, arg)
	}
	return Expression{
		operation: OperationRelatedColumn,
		target:    name,
		arguments: arguments,
		origin:    symbols.NodeOrigin(owner.DocName, invocation),
	}, nil
}
