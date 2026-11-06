package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// structureParameter is a parameter of a behavior or step the graph states, with its direction.
type structureParameter struct {
	element   Element
	direction ast.FeatureDirection
}

// structureImpliedBaseFQNs names the standard-library bases element's metaclass or
// declaration implies (SysML 8.3.6.2, KerML 8.4.4) and, for a usage, the library feature
// its nesting implies it subsets (SysML 8.3.6.4).
func (e *Evaluator) structureImpliedBaseFQNs(element Element) ([]string, bool) {
	kind := source.KindSysML
	if documentKind, known := e.elementDocumentKind(element); known {
		kind = documentKind
	}
	var bases []string
	if metaclass, known := e.options.Structure.Metaclass(element); known {
		bases = e.model.ImpliedBaseFQNsForMetaclass(metaclass, kind)
	}
	if len(bases) == 0 && element.Node != nil {
		bases = e.model.ImpliedBaseFQNs(element.Node, kind)
	}
	if kind != source.KindKerML {
		nested, ok := e.structureNestedSubsettingFQN(element)
		if !ok {
			return nil, false
		}
		if nested != "" {
			bases = append(bases, nested)
		}
	}
	return bases, true
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

// structureBehaviorGenerals returns the behaviors and steps owner specializes by its
// stated generalizations; the kind's implied base owns no parameters and is left out.
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
			implied[ElementOf(symbol).Key()] = true
		}
	}
	var generals []Element
	for _, general := range supertypes {
		if implied[general.Key()] {
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
	return generals, true
}

// structureParameterTable mirrors the semantic parameter table: owner's own positional
// parameters, then what a lone general leaves unclaimed by clause or position, and the result.
func (e *Evaluator) structureParameterTable(owner Element, visiting map[ElementKey]bool) ([]structureParameter, *structureParameter, bool) {
	if symbol := e.structureElementSymbol(owner); symbol != nil && !e.structureKnows(owner) {
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
	var positional []structureParameter
	var result *structureParameter
	for _, value := range owned.Values {
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
	generals, ok := e.structureBehaviorGenerals(owner)
	if !ok {
		return nil, nil, false
	}
	if len(generals) != 1 {
		return positional, result, true
	}
	inherited, inheritedResult, ok := e.structureParameterTable(generals[0], visiting)
	if !ok {
		return nil, nil, false
	}
	claimed := make(map[ElementKey]bool)
	for i, p := range positional {
		explicit, ok := e.structureRedefinitionTargets(p.element)
		if !ok {
			return nil, nil, false
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
			return nil, nil, false
		}
		for _, target := range explicit {
			claimed[target.Key()] = true
		}
		if inheritedResult != nil {
			claimed[inheritedResult.element.Key()] = true
		}
	}
	for _, p := range inherited {
		if !claimed[p.element.Key()] {
			positional = append(positional, p)
		}
	}
	if result == nil && inheritedResult != nil && !claimed[inheritedResult.element.Key()] {
		result = inheritedResult
	}
	return positional, result, true
}

// structureImplicitParameterRedefinitions is the parameter at feature's position (or the result)
// in each general of its owner (KerML 8.4.4.6); matchDirection keeps the agreeing directions only.
func (e *Evaluator) structureImplicitParameterRedefinitions(feature Element, matchDirection bool) ([]Element, bool) {
	if e.model == nil {
		return nil, true
	}
	owner, handled, ok := e.structureOwningType(feature)
	if !handled || !ok {
		return nil, false
	}
	if owner.Kind != ElementValue {
		return nil, true
	}
	isBehavior, known := e.isMetaclass(owner.Element, "Behavior")
	if !known {
		return nil, false
	}
	isStep, known := e.isMetaclass(owner.Element, "Step")
	if !known {
		return nil, false
	}
	if !isBehavior && !isStep {
		return nil, true
	}
	direction, ok := e.structureEffectiveDirection(owner.Element, feature)
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
		position, ok = e.structureParameterPosition(owner.Element, feature)
		if !ok {
			return nil, false
		}
		if position < 0 {
			return nil, true
		}
	}
	generals, ok := e.structureBehaviorGenerals(owner.Element)
	if !ok {
		return nil, false
	}
	var out []Element
	for _, general := range generals {
		positional, result, ok := e.structureParameterTable(general, make(map[ElementKey]bool))
		if !ok {
			return nil, false
		}
		target := result
		if !isResult {
			target = nil
			if position < len(positional) {
				target = &positional[position]
			}
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
