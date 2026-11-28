package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	selfLinksFQN  = "Links::selfLinks"
	thisThingFQN  = "Links::selfLinks::thisThing"
	sameThingFQN  = "Links::selfLinks::sameThing"
	baseThingsFQN = "Base::things"
	startShotFQN  = "Occurrences::Occurrence::startShot"
)

// impliedBinding is a BindingConnector a semantic constraint requires of its owner (KerML Table 11;
// SysML Table 33), with the ends it owns and the chain its first end refers to, if it binds one.
type impliedBinding struct {
	connector *symbols.Symbol
	ends      [2]*symbols.Symbol
	related   [2]*symbols.Symbol // what each end's ReferenceSubsetting targets
	// valued is the feature whose value the connector binds, whose featuring the connector has, and
	// initial the chain that features it instead when the value is an initial one (KerML 8.4.4.11).
	valued  *symbols.Symbol
	initial *symbols.Symbol
	// owned marks a connector its owner owns as a feature, which features it (SysML Table 33).
	owned bool
}

// impliedChain is a feature chain an implied relationship owns, with the features it chains.
type impliedChain struct {
	chain *symbols.Symbol
	links []*symbols.Symbol
}

// inclusionElement synthesizes an element held for an implied relationship of owner.
func inclusionElement(owner *symbols.Symbol, role symbols.InclusionRole, index int) *symbols.Symbol {
	return &symbols.Symbol{
		Kind:       symbols.SymbolKerMLFeature,
		DocName:    owner.DocName,
		DeclSpan:   owner.DeclSpan,
		Visibility: owner.Visibility,
		Inclusion:  &symbols.InclusionElement{Owner: owner, Role: role, Index: index},
	}
}

// ImpliedBindingConnectors returns the BindingConnectors owner owns once implied relationships
// are included, in the order of the constraints that require them (KerML 8.4.2).
func (m *Model) ImpliedBindingConnectors(owner *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, binding := range m.impliedBindingsOf(owner) {
		out = append(out, binding.connector)
	}
	return out
}

// impliedBindingsOf synthesizes owner's implied bindings once per model.
func (m *Model) impliedBindingsOf(owner *symbols.Symbol) []*impliedBinding {
	if m == nil || owner == nil || owner.Inclusion != nil || owner.Implicit != nil {
		return nil
	}
	defer m.own(owner).LeaveDoc()
	if cached, ok := m.impliedBindings[owner]; ok {
		return cached
	}
	if m.impliedBindings == nil {
		m.impliedBindings = make(map[*symbols.Symbol][]*impliedBinding)
		m.bindingOfConnector = make(map[*symbols.Symbol]*impliedBinding)
	}
	if m.impliedChains == nil {
		m.impliedChains = make(map[*symbols.Symbol]*impliedChain)
	}
	m.resolver.Enter()
	var out []*impliedBinding
	bind := func(source, target *symbols.Symbol, chained ...*symbols.Symbol) *impliedBinding {
		connector := inclusionElement(owner, symbols.InclusionBindingConnector, len(out))
		binding := &impliedBinding{connector: connector}
		for i := range binding.ends {
			binding.ends[i] = inclusionElement(connector, symbols.InclusionConnectorEnd, i)
		}
		if len(chained) > 0 {
			source = m.heldChain(binding.ends[0], chained)
		}
		binding.related = [2]*symbols.Symbol{source, target}
		out = append(out, binding)
		return binding
	}
	if value, result := m.boundValue(owner); value != nil && result != nil {
		// checkFeatureValueBindingConnector: between the feature and its value's result, reached through the value.
		binding := bind(nil, owner, value, result)
		binding.valued = owner
		if _, _, isInitial, _ := featureValueOf(owner); isInitial && owner.Expr == nil {
			// The binding of an initial value holds at the start of the featuring occurrence: that.startShot.
			that, _ := m.LookupMember(m.libSymbol(baseThingsFQN), "that")
			if start := m.libSymbol(startShotFQN); that != nil && start != nil {
				binding.initial = m.heldChain(binding.connector, []*symbols.Symbol{that, start})
			}
		}
	}
	if result := m.ExpressionResult(owner); result != nil && len(m.invocationBehaviors(owner)) > 0 {
		// checkInvocationExpressionBehaviorBindingConnector: between the invocation itself and its result.
		bind(owner, result)
	}
	if isExpressionSymbol(owner) && expressionElementMetaclassName(owner) == "FeatureReferenceExpression" {
		// checkFeatureReferenceExpressionBindingConnector: between the referent and the result.
		referents := m.expressionReferencedElements(owner, "referent")
		if result := m.ExpressionResult(owner); result != nil && len(referents) == 1 && referents[0] != nil {
			bind(referents[0], result)
		}
	}
	if expr := m.ownedResultExpression(owner); expr != nil {
		// checkFunctionResultBindingConnector, checkExpressionResultBindingConnector: from the result
		// of the result expression to the result parameter.
		if source, target := m.resultOf(expr), m.ResultParameterOf(owner); source != nil && target != nil {
			bind(source, target)
		}
	}
	if parts := m.transitionPartsOf(owner); parts != nil {
		// checkTransitionUsageSuccessionBindingConnector: between the succession and the transitionLink.
		if parts.succession != nil && parts.transitionLink != nil {
			bind(parts.succession, parts.transitionLink)
		}
		// checkTransitionUsageSourceBindingConnector: between the source and the first input parameter.
		if source := m.transitionSource(owner, parts); source != nil && len(parts.linkParameters) > 0 {
			bind(source, parts.linkParameters[0])
		}
	}
	if receiver, invocation, parameter := m.triggerReceiverBinding(owner); receiver != nil {
		// checkAcceptActionUsageReceiverBindingConnector: between the receiver and the receiver
		// parameter of the trigger invocation, reached through the invocation.
		binding := bind(receiver, nil)
		binding.owned = true
		binding.related[1] = m.heldChain(binding.ends[1], []*symbols.Symbol{invocation, parameter})
	}
	if !m.resolver.Leave() {
		return out
	}
	journal(m, m.impliedBindings, owner, owner.Decl)
	m.impliedBindings[owner] = out
	for _, binding := range out {
		journal(m, m.bindingOfConnector, binding.connector, owner.Decl)
		m.bindingOfConnector[binding.connector] = binding
	}
	return out
}

// triggerReceiverBinding returns, for an accept action whose payload a trigger invocation values, its receiver
// parameter, the invocation and the receiver parameter the invocation inherits; nil for any other symbol.
func (m *Model) triggerReceiverBinding(owner *symbols.Symbol) (receiver, invocation, parameter *symbols.Symbol) {
	var payload *symbols.Symbol
	if owner.Expr != nil {
		if owner.Expr.Role != symbols.ExpressionTriggerAction {
			return nil, nil, nil
		}
		if parts := m.acceptParts(owner); parts != nil {
			payload, receiver = parts.payload, parts.receiver
		}
	} else if symbols.AcceptPayloadOf(owner.Decl) != nil {
		for _, param := range m.controlParameters(owner) {
			switch m.nodeParameterName(owner, param) {
			case "payload":
				payload = param
			case "receiver":
				receiver = param
			}
		}
	}
	if payload == nil {
		return nil, nil, nil
	}
	invocation = m.ValueExpression(payload)
	if invocation == nil || expressionElementMetaclassName(invocation) != "TriggerInvocationExpression" {
		return nil, nil, nil
	}
	parameter = m.argumentParameter(m.ExpressionFunction(invocation), 1, false)
	if receiver == nil {
		// Without `via` the action inherits its receiver: a trigger action, which no scope holds, the one
		// of the accepter it subsets (checkAcceptActionUsageTriggerActionSpecialization).
		inheriting := owner
		if owner.Expr != nil {
			inheriting = m.libSymbol("Actions::TransitionAction::accepter")
		}
		receiver, _ = m.LookupMember(inheriting, "receiver")
	}
	if receiver == nil || parameter == nil {
		return nil, nil, nil
	}
	return receiver, invocation, parameter
}

// boundValue returns the value expression a feature is bound to and that value's result parameter;
// nil for a feature without a value and for a default value, which binds nothing (KerML 8.4.4.11).
func (m *Model) boundValue(sym *symbols.Symbol) (value, result *symbols.Symbol) {
	if sym.Expr == nil {
		if _, isDefault, _, ok := featureValueOf(sym); !ok || isDefault {
			return nil, nil
		}
	}
	value = m.ValueExpression(sym)
	if value == nil {
		return nil, nil
	}
	return value, m.resultOf(value)
}

// heldChain synthesizes the chain owner's implied relationship targets, chaining links in order.
func (m *Model) heldChain(owner *symbols.Symbol, links []*symbols.Symbol) *symbols.Symbol {
	chain := inclusionElement(owner, symbols.InclusionChain, 0)
	journal(m, m.impliedChains, chain, owner.Decl)
	m.impliedChains[chain] = &impliedChain{chain: chain, links: links}
	return chain
}

// impliedChainingFeatures returns the features an implied relationship's chain chains, in order.
func (m *Model) impliedChainingFeatures(chain *symbols.Symbol) []*symbols.Symbol {
	if held := m.impliedChains[chain]; held != nil {
		return held.links
	}
	return nil
}

// ImpliedBinding returns the ends of an implied BindingConnector and the features they refer to.
func (m *Model) ImpliedBinding(connector *symbols.Symbol) (ends, related [2]*symbols.Symbol, ok bool) {
	if m == nil || connector == nil {
		return ends, related, false
	}
	binding := m.bindingOfConnector[connector]
	if binding == nil {
		return ends, related, false
	}
	return binding.ends, binding.related, true
}

// inclusionGenerals returns the generals of the implied Specializations an element held for an
// implied relationship owns: a binding connector's Links::selfLinks, a bound chain's Base::things.
func (m *Model) inclusionGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	var fqn string
	switch owner := sym.Inclusion.Owner; {
	case sym.Inclusion.Role == symbols.InclusionBindingConnector:
		// checkBindingConnectorSpecialization (KerML Table 8).
		fqn = selfLinksFQN
	case sym.Inclusion.Role == symbols.InclusionChain && owner.Inclusion != nil &&
		owner.Inclusion.Role == symbols.InclusionConnectorEnd:
		// checkFeatureSpecialization: the chain an end refers to is a Feature of no other kind.
		fqn = baseThingsFQN
	}
	if general := m.libSymbol(fqn); fqn != "" && general != nil {
		return []*symbols.Symbol{general}
	}
	return nil
}

// inclusionRedefinitions returns the features an element held for an implied relationship
// redefines: a binding connector's ends, the ends of Links::selfLinks at their positions.
func (m *Model) inclusionRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Inclusion.Role == symbols.InclusionChainTarget {
		// checkFeatureChainExpressionTargetRedefinition, then ...SourceTargetRedefinition.
		var out []*symbols.Symbol
		for _, source := range m.ImpliedRedefinitions(sym.Inclusion.Owner) {
			// The feature nested in the `source` parameter the owner redefines (KerML 9.4.17).
			if target, ok := m.LookupMember(source, "target"); ok {
				out = append(out, target)
			}
		}
		out = append(out, m.expressionReferencedElements(sym.Inclusion.Owner.Expr.Owner, "targetFeature")...)
		return slices.DeleteFunc(out, func(feature *symbols.Symbol) bool { return feature == nil })
	}
	if sym.Inclusion.Role != symbols.InclusionConnectorEnd {
		return nil
	}
	// checkFeatureEndRedefinition (KerML Table 9).
	fqn := thisThingFQN
	if sym.Inclusion.Index == 1 {
		fqn = sameThingFQN
	}
	if end := m.libSymbol(fqn); end != nil {
		return []*symbols.Symbol{end}
	}
	return nil
}

// inclusionTypeFeaturings returns the featuring types of an element held for an implied relationship.
func (m *Model) inclusionTypeFeaturings(sym *symbols.Symbol) []FeaturingType {
	switch sym.Inclusion.Role {
	case symbols.InclusionConnectorEnd, symbols.InclusionChainTarget:
		// checkFeatureFeatureMembershipTypeFeaturing: a connector owns its ends, a parameter its nested feature.
		return []FeaturingType{m.plainFeaturingType(sym.Inclusion.Owner)}
	case symbols.InclusionBindingConnector:
		binding := m.bindingOfConnector[sym]
		switch {
		case binding != nil && binding.initial != nil:
			return []FeaturingType{{Type: binding.initial}}
		case binding != nil && binding.valued != nil:
			// checkFeatureValueBindingConnector: the featuring types of the feature with the value.
			return m.DerivedFeaturingTypes(binding.valued)
		case binding != nil && binding.owned:
			// checkFeatureFeatureMembershipTypeFeaturing: an ownedFeature, and a binding does not time vary.
			return []FeaturingType{m.plainFeaturingType(sym.Inclusion.Owner)}
		}
		// checkConnectorTypeFeaturing: no type owns an implied connector, so it has its default.
		if featuring := m.connectorDefaultFeaturingType(sym); featuring.Type != nil {
			return []FeaturingType{featuring}
		}
	}
	return nil
}

// ImpliedEndReference returns the feature the end of an implied BindingConnector refers to
// through its ReferenceSubsetting; nil for any other symbol.
func (m *Model) ImpliedEndReference(end *symbols.Symbol) *symbols.Symbol {
	if m == nil || end == nil || end.Inclusion == nil || end.Inclusion.Role != symbols.InclusionConnectorEnd {
		return nil
	}
	if binding := m.bindingOfConnector[end.Inclusion.Owner]; binding != nil {
		return binding.related[end.Inclusion.Index]
	}
	return nil
}

// resultOf returns the result parameter of an expression: the one it owns, or the one it
// inherits from its function when it owns none.
func (m *Model) resultOf(expr *symbols.Symbol) *symbols.Symbol {
	if result := m.ExpressionResult(expr); result != nil {
		return result
	}
	return m.ResultParameterOf(expr)
}

// ownedResultExpression returns the expression owner owns through a ResultExpressionMembership.
func (m *Model) ownedResultExpression(owner *symbols.Symbol) *symbols.Symbol {
	if owner.Expr != nil {
		if parts := m.expressionPartsOf(owner); parts != nil && parts.result != nil &&
			parts.result.Expr.Role == symbols.ExpressionBodyResult {
			return parts.result
		}
		return nil
	}
	if result := m.ConstraintResultMember(owner); result != nil {
		return result
	}
	for _, member := range m.synthesizedMembersOf(owner) {
		if member.Expr != nil && member.Expr.Role == symbols.ExpressionBodyResult {
			return member
		}
	}
	return nil
}

// impliedFeatures are the features sym owns for its own implied relationships.
type impliedFeatures struct {
	target    *symbols.Symbol // the feature nested in a chain expression's source parameter
	crossing  *symbols.Symbol // the chain an end's implied CrossSubsetting targets
	valuation *symbols.Symbol // the chain a valued feature or a chain expression's result subsets
}

// impliedFeaturesOf synthesizes the features sym's implied relationships own, once per model.
func (m *Model) impliedFeaturesOf(sym *symbols.Symbol) *impliedFeatures {
	if m == nil || sym == nil || sym.Inclusion != nil || sym.Implicit != nil || sym.Recorded() {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.impliedFeatures[sym]; ok {
		return cached
	}
	if m.impliedFeatures == nil {
		m.impliedFeatures = make(map[*symbols.Symbol]*impliedFeatures)
	}
	if m.impliedChains == nil {
		m.impliedChains = make(map[*symbols.Symbol]*impliedChain)
	}
	m.resolver.Enter()
	out := &impliedFeatures{}
	switch {
	case sym.Expr != nil:
		expr := sym.Expr.Owner
		if expr == nil || expressionElementMetaclassName(expr) != "FeatureChainExpression" {
			break
		}
		params := m.ExpressionParameters(expr)
		if len(params) == 0 {
			break
		}
		if sym == params[0] {
			// Table 11, Note 5: the feature nested in the first input parameter.
			out.target = inclusionElement(sym, symbols.InclusionChainTarget, 0)
		} else if nested := m.impliedFeaturesOf(params[0]); sym == m.ExpressionResult(expr) && nested != nil && nested.target != nil {
			// checkFeatureChainExpressionResultSpecialization: the chain of the source and its nested feature.
			out.valuation = m.heldChain(sym, []*symbols.Symbol{params[0], nested.target})
		}
	default:
		if cross := m.OwnedCrossFeature(sym); cross != nil && len(CrossSubsettings(sym)) == 0 {
			// checkFeatureCrossingSpecialization: the other end, then the owned cross feature (KerML 8.4.4.5.1).
			if others := m.otherEnds(sym); len(others) == 1 {
				out.crossing = m.heldChain(sym, []*symbols.Symbol{others[0], cross})
			}
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok && resolve.ValuationSpecializes(usage) {
			if dir, _ := ReflectiveDirection(sym); dir == ast.DirNone {
				// checkFeatureValuationSpecialization: the value, then its result.
				if value, result := m.boundValue(sym); value != nil && result != nil {
					out.valuation = m.heldChain(sym, []*symbols.Symbol{value, result})
				}
			}
		}
	}
	if !m.resolver.Leave() {
		return out
	}
	journal(m, m.impliedFeatures, sym, sym.Decl)
	m.impliedFeatures[sym] = out
	return out
}

// ImpliedOwnedFeatures returns the features sym owns for implied relationships once they are
// included: a nested target feature, then the chains its CrossSubsetting and Subsetting target.
func (m *Model) ImpliedOwnedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if referred := m.ImpliedEndReference(sym); referred != nil {
		if referred.Inclusion != nil && referred.Inclusion.Role == symbols.InclusionChain && referred.Inclusion.Owner == sym {
			return []*symbols.Symbol{referred}
		}
		return nil
	}
	if binding := m.bindingOfConnector[sym]; binding != nil && binding.initial != nil {
		return []*symbols.Symbol{binding.initial}
	}
	var out []*symbols.Symbol
	if held := m.impliedFeaturesOf(sym); held != nil {
		for _, feature := range []*symbols.Symbol{held.target, held.crossing, held.valuation} {
			if feature != nil {
				out = append(out, feature)
			}
		}
	}
	return out
}

// ImpliedCrossedChain returns the feature chain the implied CrossSubsetting of an end targets, or nil.
func (m *Model) ImpliedCrossedChain(end *symbols.Symbol) *symbols.Symbol {
	if held := m.impliedFeaturesOf(end); held != nil {
		return held.crossing
	}
	return nil
}

// impliedSubsettedChain returns the chain sym subsets for its value or as a chain expression's result.
func (m *Model) impliedSubsettedChain(sym *symbols.Symbol) *symbols.Symbol {
	if held := m.impliedFeaturesOf(sym); held != nil {
		return held.valuation
	}
	return nil
}

// inclusionSupertypes returns what a held feature specializes: a chain its last feature, as any
// feature chain does (KerML 8.3.3.3.4), a nested target the features it redefines.
func (m *Model) inclusionSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	switch sym.Inclusion.Role {
	case symbols.InclusionChain:
		if links := m.impliedChainingFeatures(sym); len(links) > 0 {
			return links[len(links)-1:]
		}
	case symbols.InclusionChainTarget:
		return m.inclusionRedefinitions(sym)
	}
	return append(m.inclusionGenerals(sym), m.inclusionRedefinitions(sym)...)
}

// impliedTransitionSourceEnd returns the source of the transition whose succession owns end as its
// first end, a source the transition names through a Membership (SysML 8.3.18.9); nil otherwise.
func (m *Model) impliedTransitionSourceEnd(end *symbols.Symbol) *symbols.Symbol {
	if decl, ok := symbols.ImpliedDeclOf(end); !ok || decl.Form != symbols.ImpliedTransitionSuccessionSourceEnd {
		return nil
	}
	succession := m.ownerOf(end)
	if !transitionSuccessionPart(succession) {
		return nil
	}
	if source, ok := m.transitionSourceMember(succession.Expr.Owner); ok {
		return source
	}
	return nil
}
