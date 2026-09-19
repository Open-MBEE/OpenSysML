package queryplan

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	queryBaseFQN = "DocumentQueries::Query"
	elementFQN   = "KerML::Root::Element"
)

type builtin struct {
	operation Operation
}

var builtins = map[string]builtin{
	"DocumentQueries::OwnedElements":   {OperationOwnedElements},
	"DocumentQueries::Descendants":     {OperationDescendants},
	"DocumentQueries::Ancestors":       {OperationAncestors},
	"DocumentQueries::Objects":         {OperationObjects},
	"DocumentQueries::Verdicts":        {OperationVerdicts},
	"DocumentQueries::States":          {OperationStates},
	"DocumentQueries::InState":         {OperationInState},
	"DocumentQueries::Events":          {OperationEvents},
	"DocumentQueries::RelatedElements": {OperationRelatedElements},
	"DocumentQueries::WhereType":       {OperationWhereType},
	"DocumentQueries::WhereMetadata":   {OperationWhereMetadata},
	"DocumentQueries::WhereName":       {OperationWhereName},
	"DocumentQueries::WhereFeature":    {OperationWhereFeature},
	"DocumentQueries::OrderBy":         {OperationOrderBy},
	"DocumentQueries::Project":         {OperationProject},
	"DocumentQueries::WhereRelated":    {OperationWhereRelated},
	"DocumentQueries::Except":          {OperationExcept},
	"DocumentQueries::Union":           {OperationUnion},
}

// typedExpression pairs a compiled expression with what planning knows of its
// value: possible types, the fixed model elements it names, and multiplicity.
type typedExpression struct {
	expression        Expression
	types             []*symbols.Symbol
	elements          []*symbols.Symbol
	multiplicity      Multiplicity
	nonconformingType string
}

type compileState uint8

const (
	stateUnseen compileState = iota
	stateVisiting
	stateDone
)

// compiledSignature memoizes one query's inputs, result, and the queries its
// defaults invoke.
type compiledSignature struct {
	params       []Parameter
	result       Parameter
	dependencies []string
}

type compiler struct {
	index          *symbols.Index
	model          *semantics.Model
	resolver       *resolve.Resolver
	state          map[*symbols.Symbol]compileState
	signatureState map[*symbols.Symbol]compileState
	signatures     map[*symbols.Symbol]compiledSignature
	stack          []*symbols.Symbol
	definitions    []Definition
}

// IsQueryDefinition reports whether sym specializes DocumentQueries::Query.
func IsQueryDefinition(index *symbols.Index, model *semantics.Model, sym *symbols.Symbol) bool {
	base := queryBase(index)
	return base != nil && sym != nil && sym != base &&
		sym.Kind == symbols.SymbolCalcDef && model != nil && model.Conforms(sym, base)
}

// Compile compiles entry and every query it invokes into dependency order.
func Compile(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*Program, error) {
	if index == nil || model == nil || resolver == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	base := queryBase(index)
	if base == nil {
		return nil, &Error{Kind: ErrorLibraryUnavailable}
	}
	name := symbols.FQNOf(entry)
	if entry == nil || entry == base || entry.Kind != symbols.SymbolCalcDef || !model.Conforms(entry, base) {
		return nil, &Error{Kind: ErrorNotQueryDefinition, Query: name, Origin: entry.Origin()}
	}
	c := &compiler{
		index:          index,
		model:          model,
		resolver:       resolver,
		state:          make(map[*symbols.Symbol]compileState),
		signatureState: make(map[*symbols.Symbol]compileState),
		signatures:     make(map[*symbols.Symbol]compiledSignature),
	}
	if err := c.compileDefinition(entry); err != nil {
		return nil, err
	}
	return &Program{entry: name, definitions: c.definitions}, nil
}

func queryBase(index *symbols.Index) *symbols.Symbol {
	if index == nil {
		return nil
	}
	matches := symbols.PreferDeclared(index.LookupQualified(queryBaseFQN))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

func (c *compiler) compileDefinition(sym *symbols.Symbol) error {
	switch c.state[sym] {
	case stateDone:
		return nil
	case stateVisiting:
		return c.cycleError(sym)
	}
	c.state[sym] = stateVisiting

	dependencies := make([]string, 0)
	seenDependencies := make(map[string]bool)
	dependency := func(name string) {
		if !seenDependencies[name] {
			seenDependencies[name] = true
			dependencies = append(dependencies, name)
		}
	}
	params, result, err := c.signature(sym, dependency)
	if err != nil {
		return err
	}
	c.stack = append(c.stack, sym)
	if result.Name == "" {
		return &Error{
			Kind:   ErrorMissingResultParameter,
			Query:  symbols.FQNOf(sym),
			Origin: sym.Origin(),
		}
	}
	resultExpression, err := c.resultExpression(sym)
	if err != nil {
		return err
	}
	expression, err := c.compileExpression(sym, resultExpression.owner, params, resultExpression.node, dependency)
	if err != nil {
		return err
	}

	c.stack = c.stack[:len(c.stack)-1]
	c.state[sym] = stateDone
	c.definitions = append(c.definitions, Definition{
		name:         symbols.FQNOf(sym),
		parameters:   params,
		result:       result,
		expression:   expression.expression,
		dependencies: dependencies,
		origin:       sym.Origin(),
	})
	return nil
}

// signature compiles sym's inputs and result once, recording the queries its
// defaults invoke; re-entering it through a default is a composition cycle.
func (c *compiler) signature(sym *symbols.Symbol, dependency func(string)) ([]Parameter, Parameter, error) {
	switch c.signatureState[sym] {
	case stateDone:
		cached := c.signatures[sym]
		for _, name := range cached.dependencies {
			dependency(name)
		}
		return cloneParameters(cached.params), cached.result, nil
	case stateVisiting:
		return nil, Parameter{}, c.cycleError(sym)
	}
	c.signatureState[sym] = stateVisiting
	c.stack = append(c.stack, sym)

	var dependencies []string
	record := func(name string) {
		if !slices.Contains(dependencies, name) {
			dependencies = append(dependencies, name)
		}
		dependency(name)
	}
	params, result, err := c.compileSignature(sym, record)
	if err != nil {
		return nil, Parameter{}, err
	}

	c.stack = c.stack[:len(c.stack)-1]
	c.signatureState[sym] = stateDone
	c.signatures[sym] = compiledSignature{
		params:       cloneParameters(params),
		result:       result,
		dependencies: dependencies,
	}
	return params, result, nil
}

func (c *compiler) compileSignature(sym *symbols.Symbol, dependency func(string)) ([]Parameter, Parameter, error) {
	effective := c.model.BehaviorParametersOf(sym)
	params := make([]Parameter, 0, len(effective))
	inputs := make([]*symbols.Symbol, 0, len(effective))
	var result Parameter
	for _, item := range effective {
		param := c.parameter(item.Symbol)
		if item.IsResult {
			result = param
			continue
		}
		if item.Direction != ast.DirIn {
			return nil, Parameter{}, &Error{
				Kind:      ErrorInvalidParameter,
				Query:     symbols.FQNOf(sym),
				Parameter: param.Name,
				Origin:    item.Symbol.Origin(),
			}
		}
		params = append(params, param)
		inputs = append(inputs, item.Symbol)
	}
	for i := range params {
		if err := c.compileDefault(sym, inputs[i], params, &params[i], dependency); err != nil {
			return nil, Parameter{}, err
		}
	}
	return params, result, nil
}

func (c *compiler) parameter(sym *symbols.Symbol) Parameter {
	return Parameter{
		Name:         sym.Name,
		Type:         c.parameterType(sym),
		Multiplicity: c.parameterMultiplicity(sym),
		Origin:       sym.Origin(),
	}
}

// compileDefault retains the nearest default along the parameter's lineage, so
// a redefining parameter's default wins over an inherited one. A default that
// names an element binds that element; any other form is compiled as an
// expression in the declaring query's scope.
func (c *compiler) compileDefault(
	query *symbols.Symbol,
	sym *symbols.Symbol,
	params []Parameter,
	param *Parameter,
	dependency func(string),
) error {
	for _, candidate := range c.parameterLineage(sym) {
		usage, ok := candidate.Decl.(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		owner := declaringQuery(candidate)
		if owner == nil {
			return &Error{
				Kind:      ErrorUnsupportedDefault,
				Query:     symbols.FQNOf(query),
				Parameter: param.Name,
				Origin:    candidate.Origin(),
			}
		}
		value, err := c.compileDefaultExpression(query, owner, param.Name, params, usage.Value, dependency)
		if err != nil {
			return err
		}
		if err := c.validateDefault(query, *param, value, symbols.NodeOrigin(owner.DocName, usage.Value)); err != nil {
			return err
		}
		param.HasDefault = true
		param.Default = value.expression
		param.DefaultQuery = symbols.FQNOf(owner)
		return nil
	}
	return nil
}

// validateDefault applies the planning checks of an explicit argument to a
// parameter's default; what is only known at execution stays for bindValues.
func (c *compiler) validateDefault(
	query *symbols.Symbol,
	param Parameter,
	value typedExpression,
	origin symbols.Origin,
) error {
	if actual, ok := c.valueTypeConforms(param, value); !ok {
		return &Error{
			Kind:      ErrorDefaultType,
			Query:     symbols.FQNOf(query),
			Parameter: param.Name,
			Expected:  param.Type,
			Actual:    actual,
			Origin:    origin,
		}
	}
	if !multiplicityConforms(value.multiplicity, param.Multiplicity) {
		return &Error{
			Kind:      ErrorDefaultMultiplicity,
			Query:     symbols.FQNOf(query),
			Parameter: param.Name,
			Expected:  multiplicityString(param.Multiplicity),
			Actual:    multiplicityString(value.multiplicity),
			Origin:    origin,
		}
	}
	return nil
}

// ignoreDependency discards the dependencies of an invoked query's defaults;
// that query's own compilation records them.
func ignoreDependency(string) { /* the invoking query records them */ }

// declaringQuery is the calculation whose body declares a parameter usage.
func declaringQuery(param *symbols.Symbol) *symbols.Symbol {
	if param.OwnerScope == nil {
		return nil
	}
	return param.OwnerScope.Owner()
}

func (c *compiler) compileDefaultExpression(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	parameter string,
	params []Parameter,
	node ast.Node,
	dependency func(string),
) (typedExpression, error) {
	value, err := c.compileExpression(query, owner, params, node, dependency)
	if err != nil {
		var planning *Error
		if errors.As(err, &planning) && planning.Kind == ErrorUnsupportedExpression {
			return typedExpression{}, &Error{
				Kind:      ErrorUnsupportedDefault,
				Query:     symbols.FQNOf(query),
				Parameter: parameter,
				Origin:    planning.Origin,
			}
		}
		return typedExpression{}, err
	}
	return value, nil
}

func (c *compiler) parameterType(sym *symbols.Symbol) string {
	return symbols.FQNOf(c.parameterTypeSymbol(sym))
}

func (c *compiler) parameterTypeSymbol(sym *symbols.Symbol) *symbols.Symbol {
	for _, candidate := range c.parameterLineage(sym) {
		if types := c.declaredTypes(candidate); len(types) > 0 {
			return types[0]
		}
	}
	return nil
}

// declaredTypes resolves the targets of sym's explicit typing clauses.
func (c *compiler) declaredTypes(sym *symbols.Symbol) []*symbols.Symbol {
	var types []*symbols.Symbol
	for _, relationship := range semantics.RelationshipsOf(sym) {
		if relationship == nil || relationship.Kind != ast.RelTyping || relationship.Target == nil {
			continue
		}
		target := relationship.Target
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		name, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if resolved, ok := c.resolver.ResolveQualified(sym.OwnerScope, name); ok {
			if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
				types = append(types, canonical)
			}
		}
	}
	return types
}

func (c *compiler) parameterMultiplicity(sym *symbols.Symbol) Multiplicity {
	rng := semantics.AssumedRange()
	for _, candidate := range c.parameterLineage(sym) {
		if stated, ok := c.model.MultiplicityOf(candidate); ok {
			rng = stated
			break
		}
	}
	return Multiplicity{
		Lower:         rng.Lower.Value,
		Upper:         rng.Upper.Value,
		UpperInfinite: rng.Upper.Infinite,
		Known:         rng.Lower.Known && rng.Upper.Known,
	}
}

// parameterLineage lists sym and the parameters it redefines or subsets,
// nearest first, following only parameter-to-parameter edges (never typing).
func (c *compiler) parameterLineage(sym *symbols.Symbol) []*symbols.Symbol {
	lineage := []*symbols.Symbol{sym}
	seen := map[*symbols.Symbol]bool{sym: true}
	for next := 0; next < len(lineage); next++ {
		current := lineage[next]
		typed := make(map[*symbols.Symbol]bool)
		for _, typ := range c.declaredTypes(current) {
			typed[typ] = true
		}
		for _, candidate := range c.model.DirectSupertypes(current) {
			usage, ok := candidate.Decl.(*ast.Usage)
			if !ok || usage.Direction == ast.DirNone || typed[candidate] || seen[candidate] {
				continue
			}
			seen[candidate] = true
			lineage = append(lineage, candidate)
		}
	}
	return lineage
}

type effectiveResult struct {
	node  ast.Node
	owner *symbols.Symbol
}

func (c *compiler) resultExpression(sym *symbols.Symbol) (effectiveResult, error) {
	results, err := c.effectiveResults(sym, make(map[*symbols.Symbol]bool))
	if err != nil {
		return effectiveResult{}, err
	}
	switch len(results) {
	case 0:
		return effectiveResult{}, &Error{
			Kind:   ErrorMissingResult,
			Query:  symbols.FQNOf(sym),
			Origin: sym.Origin(),
		}
	case 1:
		return results[0], nil
	default:
		return effectiveResult{}, &Error{
			Kind:   ErrorConflictingResult,
			Query:  symbols.FQNOf(sym),
			Origin: sym.Origin(),
		}
	}
}

func (c *compiler) effectiveResults(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) ([]effectiveResult, error) {
	if sym == nil || visiting[sym] {
		return nil, nil
	}
	if result, stated, err := c.declaredResult(sym); err != nil {
		return nil, err
	} else if stated {
		return []effectiveResult{result}, nil
	}

	visiting[sym] = true
	defer delete(visiting, sym)
	if sym == queryBase(c.index) {
		return nil, nil
	}
	var results []effectiveResult
	for _, general := range c.model.DirectSupertypes(sym) {
		if general == nil || general.Kind != symbols.SymbolCalcDef ||
			(general != queryBase(c.index) && !IsQueryDefinition(c.index, c.model, general)) {
			continue
		}
		inherited, err := c.effectiveResults(general, visiting)
		if err != nil {
			return nil, err
		}
		results = append(results, inherited...)
	}
	return c.mostSpecificResults(results), nil
}

func (c *compiler) mostSpecificResults(results []effectiveResult) []effectiveResult {
	var effective []effectiveResult
	for _, candidate := range results {
		keep := true
		for i := 0; i < len(effective); {
			current := effective[i]
			switch {
			case candidate.owner == current.owner || c.model.Conforms(current.owner, candidate.owner):
				keep = false
			case c.model.Conforms(candidate.owner, current.owner):
				effective = append(effective[:i], effective[i+1:]...)
				continue
			}
			i++
		}
		if keep {
			effective = append(effective, candidate)
		}
	}
	return effective
}

func (c *compiler) declaredResult(sym *symbols.Symbol) (effectiveResult, bool, error) {
	name := symbols.FQNOf(sym)
	members := declarationMembers(sym)
	statements := lower.CalcBodyWith(sym.Decl, members, sym.Scope, c.resolver)
	if len(statements) == 1 {
		if result, ok := statements[0].(lower.Return); ok && result.Value != nil {
			return effectiveResult{node: result.Value, owner: sym}, true, nil
		}
	}
	if len(statements) > 0 {
		return effectiveResult{}, false, &Error{
			Kind:   ErrorUnsupportedResult,
			Query:  name,
			Origin: sym.Origin(),
		}
	}

	var expression ast.Node
	for _, binding := range lower.ToBindings(sym.Decl, sym.Scope) {
		for i := range binding.Ends {
			if binding.Ends[i].Path != "result" {
				continue
			}
			if expression != nil {
				return effectiveResult{}, false, &Error{
					Kind:   ErrorUnsupportedResult,
					Query:  name,
					Origin: symbols.NodeOrigin(sym.DocName, binding.Decl),
				}
			}
			expression = binding.Ends[1-i].Expr
		}
	}
	if expression == nil {
		return effectiveResult{}, false, nil
	}
	return effectiveResult{node: expression, owner: sym}, true, nil
}

func declarationMembers(sym *symbols.Symbol) []ast.Node {
	switch declaration := sym.Decl.(type) {
	case *ast.Definition:
		return declaration.Members
	case *ast.Usage:
		return declaration.Members
	default:
		return nil
	}
}

func (c *compiler) compileExpression(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	node ast.Node,
	dependency func(string),
) (typedExpression, error) {
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.compileReference(query, owner, expression)
	case *ast.InvocationExpr:
		return c.compileInvocation(query, owner, params, expression, dependency)
	case *ast.SequenceExpr:
		args := make([]Argument, 0, len(expression.Elements))
		var types, elements []*symbols.Symbol
		multiplicity := Multiplicity{Known: true}
		nonconformingType := ""
		for _, element := range expression.Elements {
			value, err := c.compileExpression(query, owner, params, element, dependency)
			if err != nil {
				return typedExpression{}, err
			}
			args = append(args, Argument{Value: value.expression})
			types = append(types, value.types...)
			elements = append(elements, value.elements...)
			multiplicity = sumMultiplicity(multiplicity, value.multiplicity)
			if nonconformingType == "" {
				nonconformingType = value.nonconformingType
			}
		}
		return typedExpression{
			expression: Expression{
				operation: OperationSequence,
				arguments: args,
				origin:    symbols.NodeOrigin(owner.DocName, node),
			},
			types:             types,
			elements:          elements,
			multiplicity:      multiplicity,
			nonconformingType: nonconformingType,
		}, nil
	case *ast.LiteralString:
		return c.literalExpression(owner, node, LiteralString, expression.Value, "ScalarValues::String"), nil
	case *ast.LiteralInteger:
		return c.literalExpression(owner, node, LiteralInteger, expression.Value, "ScalarValues::Integer"), nil
	case *ast.LiteralReal:
		return c.literalExpression(owner, node, LiteralReal, expression.Value, "ScalarValues::Real"), nil
	case *ast.LiteralBool:
		return c.literalExpression(
			owner,
			node,
			LiteralBoolean,
			strconv.FormatBool(expression.Value),
			"ScalarValues::Boolean",
		), nil
	case *ast.LiteralInfinity:
		result := c.literalExpression(owner, node, LiteralInfinity, "*", "")
		result.nonconformingType = "infinity"
		return result, nil
	case *ast.NullExpr:
		result := c.literalExpression(owner, node, LiteralNull, "null", "")
		result.multiplicity = Multiplicity{Known: true}
		return result, nil
	case *ast.IndexExpr:
		if expression.Bracket {
			return c.quantityExpression(query, owner, expression)
		}
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, node),
		}
	default:
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, node),
		}
	}
}

// compileReference applies the %run-query binding rule: a query input parameter
// name reads that parameter; any other name binds the model element it denotes.
func (c *compiler) compileReference(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	expression *ast.FeatureReference,
) (typedExpression, error) {
	unknown := &Error{
		Kind:      ErrorUnknownParameter,
		Query:     symbols.FQNOf(query),
		Parameter: expression.Name.Text(),
		Origin:    symbols.NodeOrigin(owner.DocName, expression),
	}
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Name)
	if !ok || target == nil {
		return typedExpression{}, unknown
	}
	for _, param := range c.model.BehaviorParametersOf(query) {
		if !c.parameterIncludes(param.Symbol, target) {
			continue
		}
		if param.IsResult {
			return typedExpression{}, unknown
		}
		return typedExpression{
			expression: Expression{
				operation: OperationParameter,
				target:    param.Symbol.Name,
				origin:    symbols.NodeOrigin(owner.DocName, expression),
			},
			types:        symbolSlice(c.parameterTypeSymbol(param.Symbol)),
			multiplicity: c.parameterMultiplicity(param.Symbol),
		}, nil
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(target); ok {
		target = canonical
	}
	return typedExpression{
		expression: Expression{
			operation: OperationElement,
			target:    symbols.FQNOf(target),
			element:   target,
			origin:    symbols.NodeOrigin(owner.DocName, expression),
		},
		elements:     []*symbols.Symbol{target},
		multiplicity: Multiplicity{Lower: 1, Upper: 1, Known: true},
	}, nil
}

func (c *compiler) parameterIncludes(param, target *symbols.Symbol) bool {
	for _, candidate := range c.parameterLineage(param) {
		if candidate == target {
			return true
		}
	}
	return false
}

func (c *compiler) compileInvocation(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	expression *ast.InvocationExpr,
	dependency func(string),
) (typedExpression, error) {
	name := expression.Type.Text()
	if expression.Operand != nil {
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: name,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	selection := c.model.SelectCall(owner.Scope, expression, semantics.PerformsBehavior)
	if selection.Ambiguous {
		return typedExpression{}, &Error{
			Kind:   ErrorAmbiguousInvocation,
			Query:  symbols.FQNOf(query),
			Target: name,
			Path:   qualifiedNames(selection.Tied),
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	target := selection.Called()
	if target == nil {
		return typedExpression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: name,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	targetName := symbols.FQNOf(target)
	if targetName == columnFQN || targetName == relatedColumnFQN {
		return typedExpression{}, &Error{
			Kind:   ErrorInvalidColumn,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	if operation, ok := builtins[targetName]; ok {
		targetParams, targetResult, err := c.signature(target, ignoreDependency)
		if err != nil {
			return typedExpression{}, err
		}
		args, err := c.compileBuiltinArguments(
			query,
			owner,
			params,
			expression,
			targetName,
			targetParams,
			dependency,
		)
		if err != nil {
			return typedExpression{}, err
		}
		if operation.operation == OperationProject {
			if err := c.validateProject(query, owner, expression, args); err != nil {
				return typedExpression{}, err
			}
		}
		return typedExpression{
			expression: Expression{
				operation: operation.operation,
				target:    targetName,
				arguments: args,
				origin:    symbols.NodeOrigin(owner.DocName, expression),
			},
			types:        symbolSlice(c.typeSymbol(targetResult.Type)),
			multiplicity: targetResult.Multiplicity,
		}, nil
	}
	if !IsQueryDefinition(c.index, c.model, target) {
		return typedExpression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}
	if len(expression.Args) > 0 {
		return typedExpression{}, &Error{
			Kind:   ErrorPositionalQueryArgs,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Origin: symbols.NodeOrigin(owner.DocName, expression),
		}
	}

	call := symbols.NodeOrigin(owner.DocName, expression)
	closesCycle := c.signatureState[target] == stateVisiting
	targetParams, targetResult, err := c.signature(target, ignoreDependency)
	if err != nil {
		return typedExpression{}, closeCycle(err, closesCycle, call)
	}
	args, err := c.compileNamedArguments(
		query,
		owner,
		params,
		targetName,
		targetParams,
		expression,
		dependency,
	)
	if err != nil {
		return typedExpression{}, err
	}
	dependency(targetName)
	closesCycle = c.state[target] == stateVisiting
	if err := c.compileDefinition(target); err != nil {
		return typedExpression{}, closeCycle(err, closesCycle, call)
	}
	return typedExpression{
		expression: Expression{
			operation: OperationInvoke,
			target:    targetName,
			arguments: args,
			origin:    symbols.NodeOrigin(owner.DocName, expression),
		},
		types:        symbolSlice(c.typeSymbol(targetResult.Type)),
		multiplicity: targetResult.Multiplicity,
	}, nil
}

func (c *compiler) compileBuiltinArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	expression *ast.InvocationExpr,
	target string,
	targetParams []Parameter,
	dependency func(string),
) ([]Argument, error) {
	if len(expression.Args) > 0 {
		if len(expression.Args) > len(targetParams) {
			return nil, &Error{
				Kind:   ErrorArgumentCount,
				Query:  symbols.FQNOf(query),
				Target: expression.Type.Text(),
				Origin: symbols.NodeOrigin(owner.DocName, expression),
			}
		}
		// Trailing defaulted parameters may be omitted positionally.
		for _, param := range targetParams[len(expression.Args):] {
			if !param.HasDefault {
				return nil, &Error{
					Kind:   ErrorArgumentCount,
					Query:  symbols.FQNOf(query),
					Target: expression.Type.Text(),
					Origin: symbols.NodeOrigin(owner.DocName, expression),
				}
			}
		}
		args := make([]Argument, 0, len(expression.Args))
		for i, node := range expression.Args {
			if targetParams[i].Type == columnSpecFQN {
				columns, err := c.compileColumns(query, owner, params, node, dependency)
				if err != nil {
					return nil, err
				}
				args = append(args, Argument{Name: targetParams[i].Name, Value: columns})
				continue
			}
			value, err := c.compileExpression(query, owner, params, node, dependency)
			if err != nil {
				return nil, err
			}
			if err := c.validateArgument(
				query,
				target,
				targetParams[i],
				value,
				symbols.NodeOrigin(owner.DocName, node),
			); err != nil {
				return nil, err
			}
			args = append(args, Argument{Name: targetParams[i].Name, Value: value.expression})
		}
		return args, nil
	}

	return c.compileNamedArguments(
		query,
		owner,
		params,
		expression.Type.Text(),
		targetParams,
		expression,
		dependency,
	)
}

func (c *compiler) compileNamedArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	callerParams []Parameter,
	target string,
	targetParams []Parameter,
	expression *ast.InvocationExpr,
	dependency func(string),
) ([]Argument, error) {
	named := expression.NamedArgs
	bound := make(map[string]ast.Node, len(named))
	known := make(map[string]Parameter, len(targetParams))
	for _, param := range targetParams {
		known[param.Name] = param
	}
	for _, arg := range named {
		name := arg.Name.Text()
		if _, exists := bound[name]; exists {
			return nil, &Error{
				Kind:      ErrorDuplicateArgument,
				Query:     symbols.FQNOf(query),
				Target:    target,
				Parameter: name,
				Origin:    symbols.NodeOrigin(owner.DocName, arg.Value),
			}
		}
		if _, ok := known[name]; !ok {
			return nil, &Error{
				Kind:      ErrorUnknownArgument,
				Query:     symbols.FQNOf(query),
				Target:    target,
				Parameter: name,
				Origin:    symbols.NodeOrigin(owner.DocName, arg.Value),
			}
		}
		bound[name] = arg.Value
	}

	args := make([]Argument, 0, len(bound))
	for _, param := range targetParams {
		node, ok := bound[param.Name]
		if !ok {
			if !param.HasDefault {
				return nil, &Error{
					Kind:      ErrorMissingArgument,
					Query:     symbols.FQNOf(query),
					Target:    target,
					Parameter: param.Name,
					Origin:    symbols.NodeOrigin(owner.DocName, expression),
				}
			}
			continue
		}
		if param.Type == columnSpecFQN {
			columns, err := c.compileColumns(query, owner, callerParams, node, dependency)
			if err != nil {
				return nil, err
			}
			args = append(args, Argument{Name: param.Name, Named: true, Value: columns})
			continue
		}
		value, err := c.compileExpression(query, owner, callerParams, node, dependency)
		if err != nil {
			return nil, err
		}
		if err := c.validateArgument(
			query,
			target,
			param,
			value,
			symbols.NodeOrigin(owner.DocName, node),
		); err != nil {
			return nil, err
		}
		args = append(args, Argument{Name: param.Name, Named: true, Value: value.expression})
	}
	return args, nil
}

// closeCycle relocates a composition cycle to the invocation that closed it.
func closeCycle(err error, closes bool, call symbols.Origin) error {
	if planning, ok := err.(*Error); ok && planning.Kind == ErrorCompositionCycle && closes {
		planning.Origin = call
	}
	return err
}

func (c *compiler) cycleError(target *symbols.Symbol) error {
	start := 0
	for i, sym := range c.stack {
		if sym == target {
			start = i
			break
		}
	}
	path := make([]string, 0, len(c.stack)-start+1)
	for _, sym := range c.stack[start:] {
		path = append(path, symbols.FQNOf(sym))
	}
	path = append(path, symbols.FQNOf(target))
	return &Error{
		Kind:   ErrorCompositionCycle,
		Query:  symbols.FQNOf(target),
		Path:   path,
		Origin: target.Origin(),
	}
}

func (c *compiler) literalExpression(
	query *symbols.Symbol,
	node ast.Node,
	kind LiteralKind,
	value string,
	typeName string,
) typedExpression {
	return typedExpression{
		expression: Expression{
			operation: OperationLiteral,
			literal:   kind,
			value:     value,
			origin:    symbols.NodeOrigin(query.DocName, node),
		},
		types:        symbolSlice(c.typeSymbol(typeName)),
		multiplicity: Multiplicity{Lower: 1, Upper: 1, Known: true},
	}
}

// quantityExpression folds `magnitude [unit]` in the owner's scope; the value
// has no static type, so its use is checked where it is consumed.
func (c *compiler) quantityExpression(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	node *ast.IndexExpr,
) (typedExpression, error) {
	quantity, ok := c.model.EvalQuantity(owner.Scope, node)
	if !ok {
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Origin: symbols.NodeOrigin(owner.DocName, node),
		}
	}
	return typedExpression{
		expression: Expression{
			operation: OperationLiteral,
			literal:   LiteralQuantity,
			value:     quantity.String(),
			quantity:  &quantity,
			origin:    symbols.NodeOrigin(owner.DocName, node),
		},
		multiplicity: Multiplicity{Lower: 1, Upper: 1, Known: true},
	}, nil
}

func (c *compiler) validateArgument(
	query *symbols.Symbol,
	target string,
	param Parameter,
	value typedExpression,
	origin symbols.Origin,
) error {
	if actual, ok := c.valueTypeConforms(param, value); !ok {
		return &Error{
			Kind:      ErrorArgumentType,
			Query:     symbols.FQNOf(query),
			Target:    target,
			Parameter: param.Name,
			Expected:  param.Type,
			Actual:    actual,
			Origin:    origin,
		}
	}
	if !multiplicityConforms(value.multiplicity, param.Multiplicity) {
		return &Error{
			Kind:      ErrorArgumentMultiplicity,
			Query:     symbols.FQNOf(query),
			Target:    target,
			Parameter: param.Name,
			Expected:  multiplicityString(param.Multiplicity),
			Actual:    multiplicityString(value.multiplicity),
			Origin:    origin,
		}
	}
	return nil
}

// valueTypeConforms reports whether value's statically known types, fixed
// elements and literals fit param's type, naming the offender when not.
func (c *compiler) valueTypeConforms(param Parameter, value typedExpression) (string, bool) {
	want := c.typeSymbol(param.Type)
	if want == nil {
		return "", true
	}
	if value.nonconformingType != "" {
		return value.nonconformingType, false
	}
	for _, element := range value.elements {
		if !c.elementConforms(element, want) {
			return symbols.FQNOf(element), false
		}
	}
	if !c.argumentTypesConform(value.types, want) {
		return typeNames(value.types), false
	}
	return "", true
}

func (c *compiler) argumentTypesConform(actual []*symbols.Symbol, expected *symbols.Symbol) bool {
	for _, candidate := range actual {
		if candidate == nil || candidate == expected {
			continue
		}
		got, want := c.model.PrimTypeOf(candidate), c.model.PrimTypeOf(expected)
		if got != semantics.PrimUnknown && want != semantics.PrimUnknown {
			if semantics.PrimConforms(got, want) {
				continue
			}
			return false
		}
		if c.model.Conforms(candidate, expected) {
			continue
		}
		return false
	}
	return true
}

// elementConforms mirrors the executor's check of an element value against a
// parameter type: only an enumeration literal is a data value, and every
// element is an Element.
func (c *compiler) elementConforms(element, expected *symbols.Symbol) bool {
	if c.model.IsDataType(expected) {
		return c.model.LiteralConforms(element, expected)
	}
	if symbols.SameElement(element, expected) || c.model.Conforms(element, expected) {
		return true
	}
	return symbols.FQNOf(expected) == elementFQN
}

func (c *compiler) typeSymbol(name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	matches := symbols.PreferDeclared(c.index.LookupQualified(name))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

func symbolSlice(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	return []*symbols.Symbol{sym}
}

func typeNames(types []*symbols.Symbol) string {
	if len(types) == 0 {
		return "unknown"
	}
	seen := make(map[string]bool, len(types))
	names := make([]string, 0, len(types))
	for _, sym := range types {
		name := symbols.FQNOf(sym)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, " or ")
}

func sumMultiplicity(left, right Multiplicity) Multiplicity {
	if !left.Known || !right.Known {
		return Multiplicity{}
	}
	sum := Multiplicity{
		Lower: left.Lower + right.Lower,
		Known: true,
	}
	if left.UpperInfinite || right.UpperInfinite {
		sum.UpperInfinite = true
		return sum
	}
	sum.Upper = left.Upper + right.Upper
	return sum
}

func multiplicityConforms(actual, expected Multiplicity) bool {
	if !actual.Known || !expected.Known || actual.Lower < expected.Lower {
		return !actual.Known || !expected.Known
	}
	if expected.UpperInfinite {
		return true
	}
	return !actual.UpperInfinite && actual.Upper <= expected.Upper
}

func multiplicityString(multiplicity Multiplicity) string {
	if !multiplicity.Known {
		return "unknown"
	}
	upper := strconv.FormatInt(multiplicity.Upper, 10)
	if multiplicity.UpperInfinite {
		upper = "*"
	}
	return "[" + strconv.FormatInt(multiplicity.Lower, 10) + ".." + upper + "]"
}

func qualifiedNames(syms []*symbols.Symbol) []string {
	names := make([]string, len(syms))
	for i, sym := range syms {
		names[i] = symbols.FQNOf(sym)
	}
	return names
}
