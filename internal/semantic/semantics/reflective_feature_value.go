package semantics

import (
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// featureValueOf is the value part a feature's declaration writes (KerML
// 8.3.3.3.5 FeatureValue): the expression, and the `default` and `:=` of its
// operator. ok is false for a feature declared without a value, and for a
// recorded symbol, whose record keeps no expression.
func featureValueOf(sym *symbols.Symbol) (value ast.Node, isDefault, isInitial bool, ok bool) {
	if sym == nil || sym.Recorded() {
		return nil, false, false, false
	}
	u, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage || u.Value == nil {
		return nil, false, false, false
	}
	return u.Value, u.ValueIsDefault, u.ValueIsInitial, true
}

// ValueExpression is the expression element a feature's value is (KerML
// Feature::valuation.value), synthesized once per feature from the notation's
// expression node, as the RDF conversion writes it; nil for a feature without
// a value. The FeatureValue that owns it is among the feature's
// OwnedMemberships.
func (m *Model) ValueExpression(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil {
		return nil
	}
	if cached, ok := m.valueExpressions[sym]; ok {
		return cached
	}
	value, _, _, ok := featureValueOf(sym)
	if !ok {
		return nil
	}
	journal(m, m.valueExpressions, sym, sym.Decl)
	expr := &symbols.Symbol{
		Kind:       symbols.SymbolExpression,
		DocName:    sym.DocName,
		DeclSpan:   value.Span(),
		Decl:       value,
		Visibility: sym.Visibility,
		Expr:       &symbols.ExpressionElement{Owner: sym, Role: symbols.ExpressionValue},
	}
	m.valueExpressions[sym] = expr
	return expr
}

// isValueExpressionSymbol reports whether sym is the value expression a
// FeatureValue owns.
func isValueExpressionSymbol(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil && sym.Expr.Role == symbols.ExpressionValue
}

// expressionMetaclassName is the KerML metaclass of an expression element, by
// the notation's node, as the RDF conversion classifies it.
func expressionMetaclassName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.LiteralBool:
		return "LiteralBoolean"
	case *ast.LiteralString:
		return "LiteralString"
	case *ast.LiteralInteger:
		return "LiteralInteger"
	case *ast.LiteralReal:
		return "LiteralRational"
	case *ast.LiteralInfinity:
		return "LiteralInfinity"
	case *ast.NullExpr:
		return "NullExpression"
	case *ast.QualifiedName, *ast.FeatureReference, *ast.BodyExpr:
		return "FeatureReferenceExpression"
	case *ast.OperatorExpr, *ast.CastExpr, *ast.SequenceExpr, *ast.IndexExpr:
		_ = n
		return "OperatorExpression"
	case *ast.FeatureChainExpr:
		return "FeatureChainExpression"
	case *ast.InvocationExpr:
		return "InvocationExpression"
	case *ast.ConstructorExpr:
		return "ConstructorExpression"
	case *ast.CollectExpr:
		return "CollectExpression"
	case *ast.SelectExpr:
		return "SelectExpression"
	case *ast.MetadataAccessExpr:
		return "MetadataAccessExpression"
	case *ast.TimeEvent, *ast.ChangeEvent:
		return "TriggerInvocationExpression"
	}
	return "Expression"
}

// expressionValue is reflectiveFeatureValue for an expression element: the
// value a literal states (LiteralInteger::value, …) and the operator of an
// OperatorExpression; ok is false for any other feature, which the element
// answers as an ordinary feature does.
func expressionValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch n := sym.Decl.(type) {
	case *ast.LiteralInteger:
		if feature != "value" {
			break
		}
		v, ok := new(big.Int).SetString(n.Value, 10)
		if !ok {
			return symbols.FilterValue{}, false
		}
		if v.IsInt64() {
			return symbols.FilterValue{Kind: symbols.FilterValueInt, Int: v.Int64()}, true
		}
		return symbols.FilterValue{Kind: symbols.FilterValueInt, BigInt: v}, true
	case *ast.LiteralReal:
		if feature != "value" {
			break
		}
		r, ok := new(big.Rat).SetString(n.Value)
		if !ok {
			return symbols.FilterValue{}, false
		}
		return symbols.FilterValue{Kind: symbols.FilterValueRational, Rat: r}, true
	case *ast.LiteralBool:
		if feature == "value" {
			return boolValue(n.Value), true
		}
	case *ast.LiteralString:
		if feature == "value" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: n.Value}, true
		}
	case *ast.OperatorExpr:
		if feature == "operator" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: n.Operator.String()}, true
		}
	}
	return symbols.FilterValue{}, false
}
