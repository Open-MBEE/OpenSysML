package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ExpressionFunction returns the function the expression element expr invokes, which types
// it (KerML 8.3.4.8.3, checkOperatorExpressionSpecialization): nil when it names none that resolves.
func (m *Model) ExpressionFunction(expr *symbols.Symbol) *symbols.Symbol {
	if m == nil || !isExpressionSymbol(expr) || expr.Kind != symbols.SymbolExpression {
		return nil
	}
	if function := m.ExpressionNodeFunction(expr.Decl); function != nil {
		return function
	}
	if _, invocation := expr.Decl.(*ast.InvocationExpr); invocation {
		for _, function := range m.expressionReferencedElements(expr, "function") {
			if function != nil {
				return function
			}
		}
	}
	return nil
}

// ExpressionNodeFunction returns the library function the notation of node fixes: an operator's,
// a cast's, an index's, a sequence's, a feature chain's, a select's or a collect's; nil for any other node.
func (m *Model) ExpressionNodeFunction(node ast.Node) *symbols.Symbol {
	if m == nil {
		return nil
	}
	switch node := node.(type) {
	case *ast.OperatorExpr:
		return m.OperatorFunction(node.Operator)
	case *ast.CastExpr:
		return m.OperatorFunction(ast.OpAs)
	case *ast.IndexExpr:
		return m.IndexFunction(node)
	case *ast.SequenceExpr:
		return m.OperatorFunctionNamed(",")
	case *ast.FeatureChainExpr:
		return m.OperatorFunctionNamed(".")
	case *ast.SelectExpr:
		return m.libSymbol(fqnSelect)
	case *ast.CollectExpr:
		return m.libSymbol(fqnCollect)
	}
	return nil
}

// expressionParameterRedefinitions returns what an expression's parameter or result redefines in each
// general of the expression (KerML checkFeatureParameterRedefinition, checkFeatureResultRedefinition).
func (m *Model) expressionParameterRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if !isExpressionParameterSymbol(sym) || sym.Expr.Owner == nil {
		return nil
	}
	owner := sym.Expr.Owner
	if owner.Expr != nil && owner.Expr.Role == symbols.ExpressionResult {
		// KerML checkConstructorExpressionResultFeatureRedefinition: the instantiated type's feature.
		if formal, named := m.constructorArgumentFormal(sym); formal != nil && !named {
			return []*symbols.Symbol{formal}
		}
		return nil
	}
	if expressionArgumentName(sym) != nil {
		return nil
	}
	isResult := sym.Expr.Role == symbols.ExpressionResult
	var out []*symbols.Symbol
	// An expression writes no specialization: its generals are the implied ones left after pruning.
	roles := m.controlParameterRedefinitions(owner)
	for _, general := range append(slices.Clone(m.ImpliedSpecializations(owner)), roles...) {
		target := m.argumentParameter(general, expressionParameterPosition(sym), isResult)
		if target == nil && isResult && slices.Contains(roles, general) {
			// A control action's test redefines the parameter of its role, whose result the library inherits.
			target, _ = m.LookupMember(general, "result")
		}
		if target != nil && target != sym && !slices.Contains(out, target) {
			out = append(out, target)
		}
	}
	return out
}

// argumentParameter is the parameter of general an argument at position binds: the input parameter
// there, as a call binds it (KerML 7.4.9.4, 8.3.4.8.5), or the result when isResult.
func (m *Model) argumentParameter(general *symbols.Symbol, position int, isResult bool) *symbols.Symbol {
	if isResult {
		return m.ResultParameterOf(general)
	}
	if params := m.signatureInputsOf(general).params; position >= 0 && position < len(params) {
		return params[position].sym
	}
	return nil
}

// namedArgumentRedefinitions returns the features a named argument's parameter redefines by the
// name written (`f(x = 1)`, `new T(x = 1)`): a ParameterRedefinition the notation states.
func (m *Model) namedArgumentRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if !isExpressionParameterSymbol(sym) || sym.Expr.Owner == nil {
		return nil
	}
	owner := sym.Expr.Owner
	if owner.Expr != nil && owner.Expr.Role == symbols.ExpressionResult {
		if formal, named := m.constructorArgumentFormal(sym); formal != nil && named {
			return []*symbols.Symbol{formal}
		}
		return nil
	}
	name := expressionArgumentName(sym)
	if name == nil {
		return nil
	}
	var out []*symbols.Symbol
	scope := m.expressionScopeOf(owner)
	for _, general := range m.ImpliedSpecializations(owner) {
		// The name binds as a call binds it: to an input parameter of what is invoked, or to nothing.
		sig := m.signatureInputsOf(general)
		if i := m.parameterIndex(scope, sig, name); i >= 0 && sig.params[i].sym != sym && !slices.Contains(out, sig.params[i].sym) {
			out = append(out, sig.params[i].sym)
		}
	}
	return out
}

// expressionElementGenerals returns the generals an expression element has from the expression
// it is or belongs to (KerML Table 8): an expression's function, and what its result specializes.
func (m *Model) expressionElementGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Expr == nil {
		return nil
	}
	if function := m.ExpressionFunction(sym); function != nil {
		return []*symbols.Symbol{function}
	}
	if sym.Expr.Role != symbols.ExpressionResult || sym.Expr.Owner == nil {
		return nil
	}
	expr := sym.Expr.Owner
	switch expressionElementMetaclassName(expr) {
	case "FeatureReferenceExpression":
		// checkFeatureReferenceExpressionResultSpecialization: the referent.
		return m.expressionReferencedElements(expr, "referent")
	case "FeatureChainExpression":
		// checkFeatureChainExpressionResultSpecialization: the chain's last feature redefines the target feature.
		return m.expressionReferencedElements(expr, "targetFeature")
	case "ConstructorExpression":
		// checkConstructorExpressionResultSpecialization: the instantiated type.
		return m.constructorResultTypes(sym)
	case "InvocationExpression":
		// checkInvocationExpressionBehaviorResultSpecialization: an instantiated type that is no function.
		return m.invocationBehaviors(expr)
	case "SelectExpression", "IndexExpression":
		// checkSelectExpressionResultSpecialization, checkIndexExpressionResultSpecialization.
		arguments := m.ExpressionArguments(expr)
		if len(arguments) == 0 {
			return nil
		}
		first := m.ExpressionResult(arguments[0])
		if _, index := expr.Decl.(*ast.IndexExpr); index && m.Conforms(first, m.libSymbol(fqnCollection)) {
			// Indexing a Collection selects one of its elements, which is no Collection.
			return nil
		}
		return []*symbols.Symbol{first}
	}
	return nil
}

// constructorResultTypes returns the type the constructor expression of result instantiates,
// which types result (KerML checkConstructorExpressionResultSpecialization); nil for any other symbol.
func (m *Model) constructorResultTypes(result *symbols.Symbol) []*symbols.Symbol {
	if result == nil || result.Expr == nil || result.Expr.Role != symbols.ExpressionResult || result.Expr.Owner == nil {
		return nil
	}
	expr := result.Expr.Owner
	ctor, ok := expr.Decl.(*ast.ConstructorExpr)
	if !ok || expressionElementMetaclassName(expr) != "ConstructorExpression" {
		return nil
	}
	if typ := m.referencedSymbol(expr, ctor.Type); typ != nil {
		return []*symbols.Symbol{typ}
	}
	return nil
}

// expressionArgumentName is the name the argument of the expression parameter param is written
// under (`f(x = 1)`); nil for a positional argument, a type argument and a result.
func expressionArgumentName(param *symbols.Symbol) *ast.QualifiedName {
	if param.Expr.Role != symbols.ExpressionParameter {
		return nil
	}
	operands, _ := ast.ExpressionOperands(param.Expr.Owner.Decl)
	for _, operand := range operands {
		if operand.Index == param.Expr.Index {
			return operand.Name
		}
	}
	return nil
}

// expressionParameterPosition is the position of the function parameter the expression parameter
// param stands at; a type argument is the second, after the operand a unary test leaves implicit.
func expressionParameterPosition(param *symbols.Symbol) int {
	if param.Expr.Role == symbols.ExpressionParameter {
		if _, typeArgument := ast.ExpressionOperands(param.Expr.Owner.Decl); typeArgument == param.Expr.Index {
			return 1
		}
	}
	return param.Expr.Index
}

// invocationBehaviors returns the types an InvocationExpression instantiates that are neither a
// Function nor a Feature typed by one, which its result then specializes and is bound to; nil otherwise.
func (m *Model) invocationBehaviors(expr *symbols.Symbol) []*symbols.Symbol {
	if !isExpressionSymbol(expr) || expressionElementMetaclassName(expr) != "InvocationExpression" {
		return nil
	}
	var out []*symbols.Symbol
	for _, instantiated := range m.expressionReferencedElements(expr, "function") {
		if instantiated == nil || m.reflectiveMetaclassConforms(instantiated, "Function") ||
			len(m.FunctionTypes(instantiated)) > 0 {
			continue
		}
		out = append(out, instantiated)
	}
	return out
}
