package symbols

// InclusionRole is what an element held for an implied relationship is to its owner.
type InclusionRole uint8

const (
	// InclusionBindingConnector is a BindingConnector a semantic constraint requires
	// (KerML Table 11; SysML Table 33); Owner is its owning namespace, Index its position there.
	InclusionBindingConnector InclusionRole = iota
	// InclusionConnectorEnd is an end of such a connector; Owner is the connector, Index 0 or 1.
	InclusionConnectorEnd
	// InclusionChain is the feature chain an implied relationship targets and owns; Owner is the
	// element owning that relationship: a connector end, a valued feature, a result, an end feature.
	InclusionChain
	// InclusionChainTarget is the Feature nested in the source parameter of a FeatureChainExpression,
	// which redefines its target feature (KerML Table 11, Note 5); Owner is that parameter.
	InclusionChainTarget
)

// InclusionElement is the side record of an element that exists only once implied relationships
// are included (KerML 8.4.2). Such a symbol is never registered in a scope or index.
type InclusionElement struct {
	Owner *Symbol
	Role  InclusionRole
	Index int
}
