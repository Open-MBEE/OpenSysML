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
