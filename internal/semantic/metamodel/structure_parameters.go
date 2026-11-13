package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// structureParameter is a parameter of a behavior or step the graph states, with its direction.
type structureParameter struct {
	element   Element
	direction ast.FeatureDirection
}

// structureRedefinitionTargets returns the features feature's owned Redefinitions name.
func (e *Evaluator) structureRedefinitionTargets(feature Element) ([]Element, bool) {
	if symbol := e.structureElementSymbol(feature); symbol != nil && e.model != nil && !e.structureKnows(feature) {
		return elementsOf(e.model.RedefinedFeatures(symbol)), true
	}
	redefinitions, handled, ok := e.structureOwnedFilter(feature, "Redefinition", false)
	if !handled || !ok {
		return nil, false
	}
	var out []Element
	for _, redefinition := range redefinitions.Values {
		if redefinition.Kind != ElementValue {
			return nil, false
		}
		targets, ok := e.relationshipEndpoints(redefinition.Element, "redefinedFeature")
		if !ok {
			return nil, false
		}
		for _, target := range targets {
			if target.Kind != ElementValue {
				return nil, false
			}
			out = append(out, target.Element)
		}
	}
	return out, true
}

// structureBehaviorGenerals returns the behaviors and steps owner specializes, its
// kind's implied base definition included; the base feature a kind implies owns no
// parameters (those of its type are reached through the base definition) and is left out.
func (e *Evaluator) structureBehaviorGenerals(owner Element) ([]Element, bool) {
	supertypes, ok := e.structureOwnedSupertypes(owner)
	if !ok {
		return nil, false
	}
	impliedBases, ok := e.structureImpliedBaseFQNs(owner)
	if !ok {
		return nil, false
	}
	implied := make(map[ElementKey]bool)
	for _, fqn := range impliedBases {
		for _, symbol := range e.resolver.Index().LookupQualified(fqn) {
			if symbol.IsFeature() {
				implied[ElementOf(symbol).Key()] = true
			}
		}
	}
	var generals, impliedFeatures []Element
	for _, general := range supertypes {
		if implied[general.Key()] {
			impliedFeatures = append(impliedFeatures, general)
			continue
		}
		isBehavior, known := e.isMetaclass(general, "Behavior")
		if !known {
			return nil, false
		}
		isStep, known := e.isMetaclass(general, "Step")
		if !known {
			return nil, false
		}
		if isBehavior || isStep {
			generals = append(generals, general)
		}
	}
	if len(generals) > 0 {
		return generals, true
	}
	// A direct implied base feature's behavior types stand in for it, as the semantic
	// model's implied generals name them (forBehaviorGenerals); a nested subsetting's do not.
	nested, ok := e.structureNestedSubsettingFQN(owner)
	if !ok {
		return nil, false
	}
	generals, ok = e.structureImpliedBaseBehaviors(impliedFeatures, nested)
	if !ok {
		return nil, false
	}
	return uniqueStructureElements(generals, e.options.Structure), true
}

// structureImpliedBaseBehaviors is the behaviors typing the implied base features, the one
// named by the nested subsetting FQN excepted.
func (e *Evaluator) structureImpliedBaseBehaviors(bases []Element, nested string) ([]Element, bool) {
	var generals []Element
	for _, base := range bases {
		symbol := e.structureElementSymbol(base)
		if symbol == nil || (nested != "" && e.model.QualifiedNameOf(symbol) == nested) {
			continue
		}
		for _, typ := range e.model.FeatureTypes(symbol) {
			element := ElementOf(typ)
			isBehavior, known := e.isMetaclass(element, "Behavior")
			if !known {
				return nil, false
			}
			if isBehavior {
				generals = append(generals, element)
			}
		}
	}
	return generals, true
}

// structureParameterTable mirrors the semantic parameter table: owner's own positional
// parameters, then what each general leaves unclaimed by clause or position, and the result.
func (e *Evaluator) structureParameterTable(owner Element, visiting map[ElementKey]bool) ([]structureParameter, *structureParameter, bool) {
	if symbol := e.structureElementSymbol(owner); symbol != nil && !e.structureKnows(owner) {
		positional, result := e.structureSymbolParameters(symbol)
		return positional, result, true
	}
	if visiting[owner.Key()] {
		return nil, nil, true
	}
	visiting[owner.Key()] = true
	defer delete(visiting, owner.Key())

	owned, handled, ok := e.structureOwnedFeatures(owner)
	if !handled || !ok {
		return nil, nil, false
	}
	positional, result, ok := e.structureOwnedParameters(owner, owned.Values)
	if !ok {
		return nil, nil, false
	}
	generals, ok := e.structureBehaviorGenerals(owner)
	if !ok {
		return nil, nil, false
	}
	declared := positional
	for _, general := range generals {
		inherited, inheritedResult, ok := e.structureParameterTable(general, visiting)
		if !ok {
			return nil, nil, false
		}
		claimed, ok := e.structureClaimedParameters(declared, result, inherited, inheritedResult)
		if !ok {
			return nil, nil, false
		}
		for _, p := range inherited {
			if claimed[p.element.Key()] {
				continue
			}
			listed, ok := e.structureListsParameter(positional[len(declared):], p.element)
			if !ok {
				return nil, nil, false
			}
			if !listed {
				positional = append(positional, p)
			}
		}
		if result == nil && inheritedResult != nil && !claimed[inheritedResult.element.Key()] {
			result = inheritedResult
		}
	}
	return positional, result, true
}

// structureSymbolParameters is the semantic model's parameter table of a symbol-backed owner:
// its positional parameters in order, and its result parameter.
func (e *Evaluator) structureSymbolParameters(symbol *symbols.Symbol) ([]structureParameter, *structureParameter) {
	var positional []structureParameter
	for i := 0; ; i++ {
		sym, direction := e.model.PositionalParameter(symbol, i, false)
		if sym == nil {
			break
		}
		positional = append(positional, structureParameter{ElementOf(sym), direction})
	}
	var result *structureParameter
	if sym, direction := e.model.PositionalParameter(symbol, 0, true); sym != nil {
		result = &structureParameter{ElementOf(sym), direction}
	}
	return positional, result
}

// structureOwnedParameters is the directed features among owner's owned features: the
// positional ones in order, and the first result parameter.
func (e *Evaluator) structureOwnedParameters(owner Element, owned []Value) ([]structureParameter, *structureParameter, bool) {
	var positional []structureParameter
	var result *structureParameter
	for _, value := range owned {
		if value.Kind != ElementValue {
			return nil, nil, false
		}
		direction, ok := e.structureEffectiveDirection(owner, value.Element)
		if !ok {
			return nil, nil, false
		}
		if direction == ast.DirNone {
			continue
		}
		isResult, ok := e.structureIsResultParameter(value.Element)
		if !ok {
			return nil, nil, false
		}
		p := structureParameter{value.Element, direction}
		if isResult {
			if result == nil {
				result = &p
			}
			continue
		}
		positional = append(positional, p)
	}
	return positional, result, true
}

// structureClaimedParameters is the set of a general's parameters the declared ones claim:
// by redefinition clause, else by position with an agreeing direction; the result by its role.
func (e *Evaluator) structureClaimedParameters(declared []structureParameter, result *structureParameter, inherited []structureParameter, inheritedResult *structureParameter) (map[ElementKey]bool, bool) {
	claimed := make(map[ElementKey]bool)
	for i, p := range declared {
		explicit, ok := e.structureRedefinitionTargets(p.element)
		if !ok {
			return nil, false
		}
		if len(explicit) > 0 {
			for _, target := range explicit {
				claimed[target.Key()] = true
			}
			continue
		}
		if i < len(inherited) && inherited[i].direction == p.direction {
			claimed[inherited[i].element.Key()] = true
		}
	}
	if result != nil {
		explicit, ok := e.structureRedefinitionTargets(result.element)
		if !ok {
			return nil, false
		}
		for _, target := range explicit {
			claimed[target.Key()] = true
		}
		if inheritedResult != nil {
			claimed[inheritedResult.element.Key()] = true
		}
	}
	return claimed, true
}

// structureListsParameter reports whether the parameters inherited so far carry feature, as
// itself or as one redefining it, directly or transitively; the owned ones claim by position.
func (e *Evaluator) structureListsParameter(inherited []structureParameter, feature Element) (bool, bool) {
	visited := make(map[ElementKey]bool)
	var redefines func(Element) (bool, bool)
	redefines = func(p Element) (bool, bool) {
		if p.Key() == feature.Key() {
			return true, true
		}
		if visited[p.Key()] {
			return false, true
		}
		visited[p.Key()] = true
		targets, ok := e.structureRedefinitionTargets(p)
		if !ok {
			return false, false
		}
		implicit, ok := e.structureImplicitParameterRedefinitions(p, false)
		if !ok {
			return false, false
		}
		for _, target := range append(targets, implicit...) {
			if found, ok := redefines(target); !ok || found {
				return found, ok
			}
		}
		return false, true
	}
	for _, p := range inherited {
		if found, ok := redefines(p.element); !ok || found {
			return found, ok
		}
	}
	return false, true
}

// structureImplicitParameterRedefinitions is the parameter at feature's position (or the result)
// in each general of its owner (KerML 8.4.4.6); matchDirection keeps the agreeing directions only.
func (e *Evaluator) structureImplicitParameterRedefinitions(feature Element, matchDirection bool) ([]Element, bool) {
	if e.model == nil {
		return nil, true
	}
	if symbol := e.structureElementSymbol(feature); symbol != nil && !e.structureKnows(feature) {
		if matchDirection {
			return elementsOf(e.model.ImplicitParameterRedefinitions(symbol)), true
		}
		return elementsOf(e.model.MaskedParameterRedefinitions(symbol)), true
	}
	owner, behaviorLike, ok := e.structureBehaviorOwner(feature)
	if !ok {
		return nil, false
	}
	if !behaviorLike {
		return nil, true
	}
	direction, ok := e.structureEffectiveDirection(owner, feature)
	if !ok {
		return nil, false
	}
	if direction == ast.DirNone {
		return nil, true
	}
	isResult, ok := e.structureIsResultParameter(feature)
	if !ok {
		return nil, false
	}
	position := -1
	if !isResult {
		position, ok = e.structureParameterPosition(owner, feature)
		if !ok {
			return nil, false
		}
		if position < 0 {
			return nil, true
		}
	}
	generals, ok := e.structureBehaviorGenerals(owner)
	if !ok {
		return nil, false
	}
	var out []Element
	for _, general := range generals {
		target, ok := e.structureParameterAt(general, position, isResult)
		if !ok {
			return nil, false
		}
		if target == nil || target.element.Key() == feature.Key() {
			continue
		}
		if matchDirection && target.direction != direction {
			continue
		}
		out = append(out, target.element)
	}
	return out, true
}

// structureBehaviorOwner is the type owning feature; behaviorLike reports whether it is a
// Behavior or a Step, false too when feature has no owning type element.
func (e *Evaluator) structureBehaviorOwner(feature Element) (owner Element, behaviorLike, ok bool) {
	owning, handled, ok := e.structureOwningType(feature)
	if !handled || !ok {
		return Element{}, false, false
	}
	if owning.Kind != ElementValue {
		return Element{}, false, true
	}
	isBehavior, known := e.isMetaclass(owning.Element, "Behavior")
	if !known {
		return Element{}, false, false
	}
	isStep, known := e.isMetaclass(owning.Element, "Step")
	if !known {
		return Element{}, false, false
	}
	return owning.Element, isBehavior || isStep, true
}

// structureParameterAt is general's result parameter, or the one at position, nil when it
// has none there.
func (e *Evaluator) structureParameterAt(general Element, position int, isResult bool) (*structureParameter, bool) {
	positional, result, ok := e.structureParameterTable(general, make(map[ElementKey]bool))
	if !ok {
		return nil, false
	}
	if isResult {
		return result, true
	}
	if position < len(positional) {
		return &positional[position], true
	}
	return nil, true
}

// structureMaskedParameterRedefinitions is what a parameter without a clause masks for its
// owner: the general parameters at its position whatever their direction, as the semantic masking.
func (e *Evaluator) structureMaskedParameterRedefinitions(feature Element) ([]Element, bool) {
	if symbol := e.structureElementSymbol(feature); symbol != nil && e.model != nil && !e.structureKnows(feature) {
		return elementsOf(e.model.MaskedParameterRedefinitions(symbol)), true
	}
	explicit, ok := e.structureRedefinitionTargets(feature)
	if !ok {
		return nil, false
	}
	if len(explicit) > 0 {
		return nil, true
	}
	return e.structureImplicitParameterRedefinitions(feature, false)
}

// structureIsResultParameter reports whether feature is owned through a ReturnParameterMembership.
func (e *Evaluator) structureIsResultParameter(feature Element) (bool, bool) {
	membership, handled, ok := e.structureOwningFeatureMembership(feature)
	if !handled || !ok {
		return false, false
	}
	if membership.Kind != MembershipValue {
		return false, true
	}
	return e.isMetaclass(MembershipElement(membership.Membership), "ReturnParameterMembership")
}

// structureParameterPosition is feature's index among owner's owned directed non-result
// features, in owner's order, or -1 when it is not one of them.
func (e *Evaluator) structureParameterPosition(owner, feature Element) (int, bool) {
	ownedFeatures, handled, ok := e.structureOwnedFeatures(owner)
	if !handled || !ok {
		return -1, false
	}
	position := 0
	for _, value := range ownedFeatures.Values {
		if value.Kind != ElementValue {
			return -1, false
		}
		direction, ok := e.structureEffectiveDirection(owner, value.Element)
		if !ok {
			return -1, false
		}
		if direction == ast.DirNone {
			continue
		}
		isResult, ok := e.structureIsResultParameter(value.Element)
		if !ok {
			return -1, false
		}
		if isResult {
			continue
		}
		if value.Element.Key() == feature.Key() {
			return position, true
		}
		position++
	}
	return -1, true
}

// elementsOf is the elements of the symbols, in order.
func elementsOf(symbols []*symbols.Symbol) []Element {
	out := make([]Element, 0, len(symbols))
	for _, symbol := range symbols {
		out = append(out, ElementOf(symbol))
	}
	return out
}
