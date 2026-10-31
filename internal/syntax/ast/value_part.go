package ast

// ValuePartOf is the value part a declaration writes (KerML 8.3.3.3.5
// FeatureValue): the expression, and whether its operator is `default` or `:=`.
// ok is false for a node that declares no value.
func ValuePartOf(node Node) (value Node, isDefault, isInitial, ok bool) {
	switch n := node.(type) {
	case *Usage:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	case *SubjectMember:
		value, isDefault, isInitial = n.BindingExpr, n.ValueIsDefault, n.ValueIsInitial
	case *AssumeMember:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	case *RequireMember:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	}
	if value == nil {
		return nil, false, false, false
	}
	return value, isDefault, isInitial, true
}

// ExpressionOperands are the operands an expression's notation writes, in the
// order the metamodel owns them as arguments (KerML 8.4.4.9: one `in`
// parameter per operand, bound by a FeatureValue), and whether a type
// argument follows them as one more parameter. A nil operand keeps its
// position. An invocation's, a body's and a change event's arguments follow
// rules of their own and are not listed here.
func ExpressionOperands(node Node) (operands []Node, typeArgument bool) {
	switch n := node.(type) {
	case *OperatorExpr:
		return n.Operands, n.TypeRef != nil
	case *CastExpr:
		return nil, true
	case *FeatureChainExpr:
		return []Node{n.Operand}, false
	case *IndexExpr:
		return []Node{n.Operand, n.Index}, false
	case *CollectExpr:
		return []Node{n.Operand, n.Body}, false
	case *SelectExpr:
		return []Node{n.Operand, n.Body}, false
	case *SequenceExpr:
		return n.Elements, false
	case *TimeEvent:
		return []Node{n.Duration}, false
	}
	return nil, false
}
