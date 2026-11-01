package symbols

// ExpressionRole is the place an expression element holds under its owner.
type ExpressionRole uint8

const (
	// ExpressionValue is the value of a feature, owned through a FeatureValue
	// (KerML 8.3.3.3.5: Feature::valuation).
	ExpressionValue ExpressionRole = iota
	// ExpressionArgument is the operand Index of an expression, owned as the
	// value of that expression's input parameter Index.
	ExpressionArgument
	// ExpressionParameter is the input parameter Index of an expression (a
	// Feature, direction in), owned through a ParameterMembership.
	ExpressionParameter
	// ExpressionResult is the result parameter of an expression (a Feature,
	// direction out), owned through a ReturnParameterMembership.
	ExpressionResult
)

// ExpressionElement is the side record of a symbol the reflective layer
// synthesizes for an element of an expression tree: the expression a
// feature's value is, its operands, and the parameters each expression owns.
// Such symbols are never registered in a scope or index; the owning feature or
// expression and the role and index under it identify them, as the RDF
// conversion's ids do (ExpressionIRI slots `value`, `a<i>`, `in<i>`, `out`).
type ExpressionElement struct {
	Owner *Symbol        // the feature or expression that owns this element
	Role  ExpressionRole // what the element is to its owner
	Index int            // the operand or parameter position, 0 otherwise
}
