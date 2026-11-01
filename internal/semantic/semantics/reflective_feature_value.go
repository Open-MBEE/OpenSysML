package semantics

import (
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// featureValueOf is the value part a feature's declaration writes (KerML
// 8.3.3.3.5 FeatureValue): the expression, and the `default` and `:=` of its
// operator. ok is false for a feature declared without a value, and for a
// recorded symbol, whose record keeps no expression.
func featureValueOf(sym *symbols.Symbol) (value ast.Node, isDefault, isInitial bool, ok bool) {
	if sym == nil || sym.Recorded() {
		return nil, false, false, false
	}
	if isBodyParameterSymbol(sym) {
		// A body's `in x = e` parameter shares the body node; its span finds
		// the declaration, whose value the conversion writes (bodyParameter).
		for i := range sym.Decl.(*ast.BodyExpr).Params {
			if param := &sym.Decl.(*ast.BodyExpr).Params[i]; param.Span == sym.DeclSpan {
				if param.Value == nil {
					return nil, false, false, false
				}
				return param.Value, false, false, true
			}
		}
		return nil, false, false, false
	}
	return ast.ValuePartOf(sym.Decl)
}

// ValueExpression is the expression element a feature's value is (KerML
// Feature::valuation.value), synthesized once per feature from the notation's
// expression node, as the RDF conversion writes it; nil for a feature without
// a value. The FeatureValue that owns it is among the feature's
// OwnedMemberships.
func (m *Model) ValueExpression(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil {
		return nil
	}
	if sym.Expr != nil {
		// A synthesized `in` parameter binds its operand; no other
		// expression part declares a value.
		return m.expressionParameterValue(sym)
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
	if !isExpressionSymbol(sym) {
		return false
	}
	switch sym.Expr.Role {
	case symbols.ExpressionChangeCondition, symbols.ExpressionBody, symbols.ExpressionBodyResult:
		// Owned through a FeatureMembership or ResultExpressionMembership,
		// not bound as a value (changeArgument, bodyExpression).
		return false
	}
	return true
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
	case *ast.OperatorExpr, *ast.CastExpr, *ast.SequenceExpr:
		return "OperatorExpression"
	case *ast.IndexExpr:
		// `x[i]` is the `[` operator; only `x#(i)` is an IndexExpression.
		if n.Bracket {
			return "OperatorExpression"
		}
		return "IndexExpression"
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
		v, err := ParseRational(n.Value, DefaultMaxIntegerBits)
		if err != nil {
			return symbols.FilterValue{}, false
		}
		return symbols.FilterValue{Kind: symbols.FilterValueRational, Rat: v.Rat()}, true
	case *ast.LiteralBool:
		if feature == "value" {
			return boolValue(n.Value), true
		}
	case *ast.LiteralString:
		if feature == "value" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: source.StringValue(n.Value)}, true
		}
	case *ast.OperatorExpr:
		if feature == "operator" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: n.Operator.String()}, true
		}
	case *ast.CastExpr:
		if feature == "operator" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: ast.OpAs.String()}, true
		}
	case *ast.IndexExpr:
		if feature == "operator" {
			operator := "#"
			if n.Bracket {
				operator = "["
			}
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: operator}, true
		}
	case *ast.SequenceExpr:
		if feature == "operator" {
			return symbols.FilterValue{Kind: symbols.FilterValueString, Str: ","}, true
		}
	}
	return symbols.FilterValue{}, false
}
