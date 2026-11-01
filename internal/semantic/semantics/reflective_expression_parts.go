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
	defer m.own(expr).LeaveDoc()
	if parts, ok := m.expressionParts[expr]; ok {
		return parts
	}
	journal(m, m.expressionParts, expr, expr.Decl)
	parts := &expressionParts{}
	operands, typeArgument := ast.ExpressionOperands(expr.Decl)
	for _, operand := range operands {
		if operand.Index < 0 {
			// A receiver no function is named for is an operand the
			// conversion writes without a parameter binding it.
			continue
		}
		parameter := m.expressionParameter(expr, symbols.ExpressionParameter, operand.Index)
		parts.params = append(parts.params, parameter)
		parts.operands = append(parts.operands, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       operand.Node,
			DeclSpan:   operand.Node.Span(),
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: parameter, Role: symbols.ExpressionArgument, Index: operand.Index},
		})
	}
	if typeArgument >= 0 {
		parts.params = append(parts.params, m.expressionParameter(expr, symbols.ExpressionParameter, typeArgument))
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
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.memberships[sym]; ok {
		return cached
	}
	journal(m, m.memberships, sym, sym.Decl)
	var out []*symbols.Symbol
	owning := func(member *symbols.Symbol) {
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   member.DeclSpan,
			Visibility: sym.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitMembership, Member: member,
			},
		})
	}
	reference := func(slot string, name *ast.QualifiedName) {
		target := m.referencedSymbol(name)
		if target == nil {
			return
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   sym.DeclSpan,
			Visibility: sym.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitReferenceMembership, Member: target, Slot: slot,
			},
		})
	}
	if isExpressionSymbol(sym) {
		// The conversion's order: a reference's referent and an invocation's
		// function lead; a chain's target follows its operand; the result ends.
		slot, name, leads := expressionReference(sym.Decl)
		if leads {
			reference(slot, name)
		}
		for _, parameter := range m.ExpressionParameters(sym) {
			owning(parameter)
		}
		if !leads && name != nil {
			reference(slot, name)
		}
		if result := m.ExpressionResult(sym); result != nil {
			owning(result)
		}
	} else if value := m.expressionParameterValue(sym); value != nil {
		owning(value)
	}
	m.memberships[sym] = out
	return out
}

// expressionReference is the element an expression names by a plain Membership,
// at the slot the conversion writes it: a reference's referent, an invocation's
// function (leading its parameters), a chain's target (following its operand).
// A dotted name owns a chain feature instead and is not listed here.
func expressionReference(node ast.Node) (slot string, name *ast.QualifiedName, leads bool) {
	switch n := node.(type) {
	case *ast.QualifiedName:
		return "referent", n, true
	case *ast.FeatureReference:
		return "referent", n.Name, true
	case *ast.InvocationExpr:
		if n.Type != nil && !ast.QualifiedNameHasChain(n.Type) {
			return "function", n.Type, true
		}
	case *ast.ConstructorExpr:
		if n.Type != nil && !ast.QualifiedNameHasChain(n.Type) {
			return "function", n.Type, true
		}
	case *ast.FeatureChainExpr:
		if n.Member != nil && !ast.QualifiedNameHasChain(n.Member) {
			return "targetFeature", n.Member, false
		}
	case *ast.MetadataAccessExpr:
		// The conversion files a metadata access's referenced element at `function`.
		return "function", n.Ref, true
	}
	return "", nil, false
}

// referencedSymbol is what the resolver bound name's last segment to, nil
// while it is unresolved.
func (m *Model) referencedSymbol(name *ast.QualifiedName) *symbols.Symbol {
	if name == nil || len(name.Parts) == 0 {
		return nil
	}
	sym, ok := m.resolver.PartSymbol(name, len(name.Parts)-1)
	if !ok {
		return nil
	}
	return sym
}

// expressionReferencedElements is the element the expression's reference
// membership at the slot named feature holds, read from its owned memberships.
func (m *Model) expressionReferencedElements(expr *symbols.Symbol, feature string) []*symbols.Symbol {
	for _, membership := range m.OwnedMemberships(expr) {
		if rel := membership.Implicit; rel != nil && rel.Form == symbols.ImplicitReferenceMembership && rel.Slot == feature {
			return []*symbols.Symbol{rel.Member}
		}
	}
	return []*symbols.Symbol{}
}

// referenceMembershipElements is ReflectiveElements for a plain Membership an
// expression owns: it relates the expression to the element it names and owns
// nothing (KerML 8.3.2.5 Membership; ownedMemberElement is OwningMembership's).
func referenceMembershipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	rel := sym.Implicit
	switch feature {
	case "source", "membershipOwningNamespace", "owningRelatedElement", "owner":
		return []*symbols.Symbol{rel.Owner}, true
	case "target", "memberElement":
		return []*symbols.Symbol{rel.Member}, true
	case "relatedElement":
		return []*symbols.Symbol{rel.Owner, rel.Member}, true
	case "ownedRelatedElement", "ownedRelationship", "ownedElement", "ownedAnnotation", "owningRelationship",
		"owningMembership", "owningNamespace", "documentation", "textualRepresentation", "ownedImport", "ownedMembership":
		return []*symbols.Symbol{}, true
	}
	return nil, false
}

// referenceMembershipValue is reflectiveFeatureValue for a plain Membership an
// expression owns: its member's names, and the public visibility the
// conversion leaves unstated (KerML 8.3.2.5 Membership::visibility default).
func (m *Model) referenceMembershipValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "memberName":
		return stringOrEmpty(SimpleSymbolName(sym.Implicit.Member)), true
	case "memberShortName":
		return stringOrEmpty(m.EffectiveShortNameOf(sym.Implicit.Member)), true
	case "visibility":
		return m.visibilityValue(ast.VisibilityPublic), true
	}
	return symbols.FilterValue{}, false
}
