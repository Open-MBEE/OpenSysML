package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// expressionParts are the elements an expression owns as the KerML metamodel
// reads it (KerML 8.3.4.8 Expression; 8.4.4.9 operator expressions): one `in`
// parameter per operand, each binding its operand through a FeatureValue, an
// extra `in` parameter typed by a type argument, and the `out` result
// parameter of an expression whose evaluation yields a value. The conversion
// writes exactly these (rdf_expr.go expressionOperandOwnership, typeArgument,
// resultParameter), so the reflective read synthesizes them in the same order.
type expressionParts struct {
	params   []*symbols.Symbol
	operands []*symbols.Symbol // operands[i] is the value of params[i]; nil for a type argument
	result   *symbols.Symbol
}

// resultBearingExpressions are the expression metaclasses whose evaluation
// yields a value the `out` parameter holds, as the conversion classifies them.
var resultBearingExpressions = map[string]bool{
	"FeatureReferenceExpression": true, "FeatureChainExpression": true,
	"OperatorExpression": true, "IndexExpression": true,
	"InvocationExpression": true, "ConstructorExpression": true,
	"CollectExpression": true, "SelectExpression": true,
	"TriggerInvocationExpression": true,
}

// isExpressionSymbol reports whether sym is an expression element the
// reflective read synthesized: a value or an operand.
func isExpressionSymbol(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil &&
		(sym.Expr.Role == symbols.ExpressionValue || sym.Expr.Role == symbols.ExpressionArgument)
}

// isExpressionParameterSymbol reports whether sym is a parameter the
// reflective read synthesized for an expression: an `in` or the `out`.
func isExpressionParameterSymbol(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil &&
		(sym.Expr.Role == symbols.ExpressionParameter || sym.Expr.Role == symbols.ExpressionResult)
}

// expressionPartsOf are the parts of the expression element expr, synthesized
// once per model so every read answers the same identities.
func (m *Model) expressionPartsOf(expr *symbols.Symbol) *expressionParts {
	if m == nil || !isExpressionSymbol(expr) {
		return nil
	}
	if parts, ok := m.expressionParts[expr]; ok {
		return parts
	}
	parts := &expressionParts{}
	operands, typeArgument := ast.ExpressionOperands(expr.Decl)
	for i, operand := range operands {
		if operand == nil {
			continue
		}
		parts.params = append(parts.params, m.expressionParameter(expr, symbols.ExpressionParameter, i))
		parts.operands = append(parts.operands, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       operand,
			DeclSpan:   operand.Span(),
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: parts.params[len(parts.params)-1], Role: symbols.ExpressionArgument, Index: i},
		})
	}
	if typeArgument {
		parts.params = append(parts.params, m.expressionParameter(expr, symbols.ExpressionParameter, len(operands)))
		parts.operands = append(parts.operands, nil)
	}
	if resultBearingExpressions[expressionMetaclassName(expr.Decl)] {
		parts.result = m.expressionParameter(expr, symbols.ExpressionResult, 0)
	}
	m.expressionParts[expr] = parts
	return parts
}

func (m *Model) expressionParameter(expr *symbols.Symbol, role symbols.ExpressionRole, index int) *symbols.Symbol {
	return &symbols.Symbol{
		Kind:       symbols.SymbolKerMLFeature,
		DocName:    expr.DocName,
		Decl:       expr.Decl,
		DeclSpan:   expr.DeclSpan,
		Visibility: expr.Visibility,
		Expr:       &symbols.ExpressionElement{Owner: expr, Role: role, Index: index},
	}
}

// ExpressionParameters are the `in` parameters of the expression element expr,
// one per operand and one per type argument, in the conversion's order.
func (m *Model) ExpressionParameters(expr *symbols.Symbol) []*symbols.Symbol {
	if parts := m.expressionPartsOf(expr); parts != nil {
		return parts.params
	}
	return nil
}

// ExpressionArguments are the operands of the expression element expr, as
// expression elements; each is the value of the parameter at its position.
func (m *Model) ExpressionArguments(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, operand := range parts.operands {
		if operand != nil {
			out = append(out, operand)
		}
	}
	return out
}

// ExpressionResult is the `out` parameter of the expression element expr, nil
// for an expression whose evaluation yields none (a literal, a reference).
func (m *Model) ExpressionResult(expr *symbols.Symbol) *symbols.Symbol {
	if parts := m.expressionPartsOf(expr); parts != nil {
		return parts.result
	}
	return nil
}

// expressionParameterValue is the operand the synthesized `in` parameter
// binds, nil for a type argument's parameter or the `out`.
func (m *Model) expressionParameterValue(param *symbols.Symbol) *symbols.Symbol {
	if m == nil || param == nil || param.Expr == nil || param.Expr.Role != symbols.ExpressionParameter {
		return nil
	}
	parts := m.expressionPartsOf(param.Expr.Owner)
	if parts == nil {
		return nil
	}
	for i, p := range parts.params {
		if p == param {
			return parts.operands[i]
		}
	}
	return nil
}

// expressionOwnedParts are the elements expr owns, parameters first then the
// result, as the conversion lists them.
func (m *Model) expressionOwnedParts(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return nil
	}
	out := append([]*symbols.Symbol{}, parts.params...)
	if parts.result != nil {
		out = append(out, parts.result)
	}
	return out
}

func expressionResultList(m *Model, expr *symbols.Symbol) []*symbols.Symbol {
	if result := m.ExpressionResult(expr); result != nil {
		return []*symbols.Symbol{result}
	}
	return nil
}

// expressionMemberships are the memberships an expression part owns, as the
// conversion writes them: an expression's ParameterMemberships in operand
// order then its ReturnParameterMembership; an `in` parameter's FeatureValue.
func (m *Model) expressionMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	if cached, ok := m.memberships[sym]; ok {
		return cached
	}
	var members []*symbols.Symbol
	if isExpressionSymbol(sym) {
		members = m.expressionOwnedParts(sym)
	} else if value := m.expressionParameterValue(sym); value != nil {
		members = []*symbols.Symbol{value}
	}
	out := make([]*symbols.Symbol, 0, len(members))
	for ordinal, member := range members {
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   member.DeclSpan,
			Visibility: sym.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: ordinal, Form: symbols.ImplicitMembership, Member: member,
			},
		})
	}
	m.memberships[sym] = out
	return out
}
