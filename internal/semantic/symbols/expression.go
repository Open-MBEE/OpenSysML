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
	// ExpressionChangeWrapper is the FeatureReferenceExpression a change
	// trigger's `in` parameter binds, whose referent is the written condition
	// (SysML 8.3.16.4 TriggerInvocationExpression, `when`); Owner is the parameter.
	ExpressionChangeWrapper
	// ExpressionChangeCondition is the condition a change wrapper owns through
	// a FeatureMembership and refers to; Owner is the wrapper.
	ExpressionChangeCondition
	// ExpressionBody is the Expression a `{ … }` body writes, which the
	// FeatureReferenceExpression owning it refers to (KerMLExpressions
	// BodyExpression); Owner is that reference, Decl the body node.
	ExpressionBody
	// ExpressionBodyResult is a body's result expression, owned through a
	// ResultExpressionMembership; Owner is the body.
	ExpressionBodyResult
	// ExpressionChainFeature is the Feature a dotted function or target name
	// reaches through, owned by the expression at Slot (KerML 8.3.3.3.3).
	ExpressionChainFeature
	// ExpressionGuard is a transition's guard, owned through a TransitionFeatureMembership
	// of kind guard; Owner is the transition.
	ExpressionGuard
	// ExpressionTriggerAction is the AcceptActionUsage a transition's trigger declares,
	// owned through a TransitionFeatureMembership of kind trigger; Owner is the transition.
	ExpressionTriggerAction
	// ExpressionPayload is a parameter of that accept: its payload, valued by a time or change
	// event's TriggerInvocationExpression, or its `via` receiver; Owner is the accept.
	ExpressionPayload
	// ExpressionOperand is an expression an action node statement writes at a slot of its own
	// (the `if` condition, an assignment's target and value, `send` payload and receiver, ...).
	ExpressionOperand
	// ExpressionTransitionSuccession is the succession a transition's `then` clause owns (SysML.xtext
	// TransitionSuccessionMember); ExpressionConnectorEnd its ends, Index 0 the empty source, 1 the target.
	ExpressionTransitionSuccession
	ExpressionConnectorEnd
	// ExpressionTransitionParameter is an implied transition parameter (SysML v2 §8.3.16.15): Slot
	// "linkparam" for the transitionLinkSource parameter, "triggerparam" for the trigger payload's.
	ExpressionTransitionParameter
	// ExpressionBound is a bound of a written MultiplicityRange, owned by the
	// range through an OwningMembership: Slot "lowerBound" or "upperBound".
	ExpressionBound
	// ExpressionFilterPackage is the Package an import with a filter, `import X::*[c]`,
	// imports and owns (SysML.xtext FilterPackage); Owner is the importing namespace.
	ExpressionFilterPackage
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
	Slot  string         // the position the conversion names a chain feature by
}
