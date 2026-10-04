package metamodel

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func (e *Evaluator) structureProperty(element Element, class, name string) (Value, bool, bool) {
	structure := e.options.Structure
	if structure == nil {
		return Value{}, false, false
	}
	if _, ok := structure.Metaclass(element); !ok {
		if element.Aspect == "api-root" {
			return Value{}, false, false
		}
		if element.Symbol == nil && element.Node == nil && element.Membership.Node == nil &&
			element.Membership.Symbol == nil && element.Membership.Member == nil {
			return Value{}, true, false
		}
		return Value{}, false, false
	}
	if expected, ok := typedDefinitionMetaclasses[name]; ok {
		return e.structureTypedDefinition(element, expected, typedDefinitionMany[name])
	}
	if class == "Usage" && name == "definition" {
		return e.structureTypedDefinition(element, "Definition", false)
	}
	if class == "Import" && name == "importedElement" {
		value, ok := e.importProperty(element, name)
		return value, true, ok
	}
	if structure.Specializes(class, "Membership") {
		return e.structureMembershipProperty(element, class, name)
	}
	switch class {
	case "Element":
		switch name {
		case "name", "shortName":
			return e.structureName(element, name)
		case "qualifiedName":
			return e.structureQualifiedName(element, make(map[ElementKey]bool))
		case "owner":
			return e.structureOwner(element)
		case "owningRelationship":
			return e.structureOptional(element)
		case "owningAnnotatingRelationship":
			return e.structureOwningAnnotatingRelationship(element)
		case "owningMembership":
			return e.structureOwningMembership(element)
		case "owningNamespace":
			return e.structureOwningNamespace(element)
		case "ownedElement":
			return e.structureOwnedElements(element)
		case "ownedRelationship":
			return e.structureOwnedRelationshipValues(element)
		case "ownedAnnotation":
			return e.structureOwnedFilter(element, "Annotation", false)
		case "documentation":
			return e.structureOwnedFilter(element, "Documentation", false)
		case "textualRepresentation":
			return e.structureOwnedFilter(element, "TextualRepresentation", false)
		case "isImpliedIncluded":
			return e.structureAttribute(element, name)
		}
	case "Namespace":
		switch name {
		case "ownedMembership":
			return e.structureOwnedFilter(element, "Membership", true)
		case "ownedMember":
			return e.structureOwnedMembers(element)
		case "ownedImport":
			return e.structureOwnedFilter(element, "Import", false)
		case "importedMembership":
			return e.structureImportedMemberships(element)
		case "membership":
			return e.structureNamespaceMemberships(element)
		case "member":
			return e.structureNamespaceMembers(element)
		}
	case "Type":
		switch name {
		case "ownedFeature":
			return e.structureOwnedFeatures(element)
		case "ownedFeatureMembership":
			return e.structureOwnedFilter(element, "FeatureMembership", true)
		case "featureMembership":
			return e.structureEffectiveFeatureMemberships(element)
		case "feature":
			return e.structureEffectiveFeatures(element)
		case "inheritedFeature":
			return e.structureInheritedFeatures(element)
		case "inheritedMembership":
			return e.structureInheritedMemberships(element)
		case "input", "output", "directedFeature":
			return e.structureDirectedFeatures(element, name)
		case "multiplicity":
			multiplicities, handled, ok := e.structureOwnedFilter(element, "MultiplicityRange", false)
			if !handled || !ok {
				return Value{}, true, false
			}
			return structurePropertyCardinality(multiplicities.Values, true)
		case "ownedEndFeature":
			return e.structureOwnedEndFeatures(element)
		case "endFeature":
			return e.structureEffectiveEndFeatures(element)
		case "ownedConjugator":
			conjugators, handled, ok := e.structureOwnedFilter(element, "Conjugation", false)
			if !handled || !ok {
				return Value{}, true, false
			}
			if len(conjugators.Values) == 0 {
				return nullValue(), true, true
			}
			if len(conjugators.Values) != 1 {
				return Value{}, true, false
			}
			return conjugators.Values[0], true, true
		case "differencingType", "intersectingType", "unioningType":
			return e.structureTypeOperands(element, name)
		case "isConjugated":
			return e.structureIsConjugated(element)
		}
		if strings.HasPrefix(name, "owned") {
			if rangeClass, ok := structureRange(class, name); ok && rangeClass != "Feature" {
				return e.structureOwnedFilter(element, rangeClass, false)
			}
		}
	case "Feature":
		switch name {
		case "owningFeatureMembership":
			return e.structureOwningFeatureMembership(element)
		case "owningType":
			return e.structureOwningType(element)
		case "endOwningType":
			return e.structureEndOwningType(element)
		case "chainingFeature":
			return e.structureFeatureChaining(element)
		case "featureTarget":
			return e.structureFeatureTarget(element)
		case "type":
			return e.structureFeatureTypes(element)
		case "crossFeature":
			return e.structureCrossFeature(element)
		case "featuringType":
			return e.structureFeaturingTypes(element, make(map[ElementKey]bool))
		case "direction":
			if value, ok := structure.Attribute(element, name); ok {
				if value.Kind != NullValue {
					return value, true, true
				}
			}
			direction, ok := e.featureDirection(element)
			if !ok {
				return Value{}, true, false
			}
			if direction == ast.DirNone {
				return nullValue(), true, true
			}
			return enumValue(direction.String()), true, true
		case "isComposite", "isVariable":
			return e.structureAttribute(element, name)
		}
		if strings.HasPrefix(name, "owned") {
			if rangeClass, ok := structureRange(class, name); ok {
				return e.structureOwnedFilter(element, rangeClass, false)
			}
		}
	case "AnnotatingElement":
		switch name {
		case "annotation":
			return e.structureOwnedFilter(element, "Annotation", false)
		case "annotatedElement":
			return e.structureAnnotatedElements(element)
		}
		if strings.HasPrefix(name, "owned") {
			if rangeClass, ok := structureRange(class, name); ok && rangeClass != "Feature" {
				return e.structureOwnedFilter(element, rangeClass, false)
			}
		}
	case "Usage":
		switch name {
		case "owningUsage":
			return e.structureOwningTypeOf(element, "Usage")
		case "owningDefinition":
			return e.structureOwningTypeOf(element, "Definition")
		case "usage":
			return e.structureFilteredEffectiveFeatures(element, "Usage")
		case "directedUsage":
			return e.structureDirectedUsages(element)
		case "variant", "variantMembership":
			return e.structureVariantProperty(element, name)
		case "isReference", "mayTimeVary":
			if value, ok := structure.Attribute(element, name); ok {
				return value, true, true
			}
			return e.semanticBooleanAttribute(element, name)
		}
		if strings.HasPrefix(name, "nested") {
			return e.structureFilteredOwnedFeatures(element, name, "nested")
		}
	case "Definition":
		switch name {
		case "usage":
			return e.structureFilteredEffectiveFeatures(element, "Usage")
		case "directedUsage":
			return e.structureDirectedUsages(element)
		case "variant", "variantMembership":
			return e.structureVariantProperty(element, name)
		}
		if _, supported := e.ownedUsageMetaclass(name); supported {
			return e.structureFilteredOwnedFeatures(element, name, "owned")
		}
	case "ActionDefinition":
		if name == "action" {
			return e.structureFilteredEffectiveFeatures(element, "ActionUsage")
		}
	case "Classifier":
		if name == "ownedSubclassification" {
			return e.structureOwnedFilter(element, "Subclassification", false)
		}
	case "Behavior":
		switch name {
		case "step":
			return e.structureFilteredEffectiveFeatures(element, "Step")
		case "parameter":
			return e.structureBehaviorParameters(element)
		}
	case "Step":
		switch name {
		case "behavior":
			return e.structureTypedFeatureTypes(element, "Behavior", false)
		case "parameter":
			return e.structureStepParameters(element)
		}
	case "CaseDefinition", "CaseUsage":
		return e.structureCaseProperty(element, name)
	case "RequirementDefinition", "RequirementUsage":
		return e.structureRequirementProperty(element, name)
	case "StateDefinition", "StateUsage":
		return e.structureStateProperty(element, name)
	case "TransitionUsage":
		return e.structureTransitionProperty(element, name)
	case "Flow":
		return e.structureFlowProperty(element, name)
	case "Expression":
		switch name {
		case "function":
			if instantiation, known := e.isMetaclass(element, "InstantiationExpression"); !known {
				return Value{}, true, false
			} else if instantiation {
				return Value{}, true, false
			}
			return e.structureTypedFeatureTypes(element, "Function", true)
		case "result":
			return e.structureResultFeatures(element)
		case "isModelLevelEvaluable":
			return e.structureAttribute(element, name)
		}
	case "InstantiationExpression":
		if name == "instantiatedType" {
			value, ok := e.instantiationExpressionProperty(element, name)
			return value, true, ok
		}
	case "Function":
		switch name {
		case "expression":
			return e.structureFilteredEffectiveFeatures(element, "Expression")
		case "result":
			return e.structureResultFeatures(element)
		case "isModelLevelEvaluable":
			return e.structureAttribute(element, name)
		}
	case "Connector":
		switch name {
		case "connectorEnd":
			return e.structureEffectiveEndFeatures(element)
		case "relatedFeature":
			return e.structureRelationshipProperty(element, class, "relatedElement")
		case "sourceFeature":
			return e.structureModelRelatedValues(element, name, "Feature", true)
		case "targetFeature":
			return e.structureModelRelatedValues(element, name, "Feature", false)
		case "association":
			return e.structureTypedFeatureTypes(element, "Association", false)
		case "defaultFeaturingType":
			return e.structureFeaturingTypes(element, make(map[ElementKey]bool))
		default:
			return e.structureRelationshipProperty(element, class, name)
		}
	case "MultiplicityRange":
		switch name {
		case "bound":
			return e.structureMultiplicityBounds(element, name)
		case "lowerBound", "upperBound":
			return e.structureMultiplicityBounds(element, name)
		}
	}
	if structure.Specializes(class, "Relationship") {
		return e.structureRelationshipProperty(element, class, name)
	}
	if strings.HasPrefix(name, "owned") {
		if rangeClass, ok := structureRange(class, name); ok {
			return e.structureOwnedFilter(element, rangeClass, false)
		}
	}
	return Value{}, false, false
}

func (e *Evaluator) structureMembershipProperty(element Element, class, name string) (Value, bool, bool) {
	switch name {
	case "memberName":
		return e.structureMemberName(element, "memberName", "declaredName")
	case "memberShortName":
		return e.structureMemberName(element, "memberShortName", "declaredShortName")
	case "ownedMemberName":
		return e.structureMemberName(element, "", "declaredName")
	case "ownedMemberShortName":
		return e.structureMemberName(element, "", "declaredShortName")
	case "memberElement", "ownedMemberElement", "ownedMemberFeature",
		"ownedMemberParameter", "ownedActorParameter", "ownedSubjectParameter",
		"ownedResultExpression", "ownedVariantUsage", "ownedConstraint",
		"ownedConcern", "action", "transitionFeature":
		expected := map[string]string{
			"ownedMemberFeature":    "Feature",
			"ownedMemberParameter":  "Feature",
			"ownedActorParameter":   "PartUsage",
			"ownedSubjectParameter": "Usage",
			"ownedResultExpression": "Expression",
			"ownedVariantUsage":     "Usage",
			"ownedConstraint":       "ConstraintUsage",
			"ownedConcern":          "ConcernUsage",
			"action":                "ActionUsage",
			"transitionFeature":     "Step",
		}[name]
		members, ok := e.structureMembershipMembers(element)
		if !ok {
			return Value{}, true, false
		}
		if expected == "" {
			return structureSingle(members), true, true
		}
		filtered, ok := e.structureFilterElements(members, expected)
		if !ok {
			return Value{}, true, false
		}
		values := make([]Value, 0, len(filtered))
		for _, member := range filtered {
			values = append(values, member)
		}
		return structurePropertyCardinality(values, true)
	case "owningType":
		return e.structureRelationshipOwner(element, "Type")
	}
	return e.structureRelationshipProperty(element, class, name)
}

func (e *Evaluator) structureFeatureChaining(element Element) (Value, bool, bool) {
	relationships, handled, ok := e.structureOwnedFilter(element, "FeatureChaining", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, relationship := range relationships.Values {
		endpoints, ok := e.relationshipEndpoints(relationship.Element, "chainingFeature")
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(endpoints, "Feature")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, filtered...)
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureCrossFeature(element Element) (Value, bool, bool) {
	relationships, handled, ok := e.structureOwnedFilter(element, "CrossSubsetting", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, relationship := range relationships.Values {
		endpoints, ok := e.relationshipEndpoints(relationship.Element, "crossedFeature")
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(endpoints, "Feature")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, filtered...)
	}
	return structurePropertyCardinality(values, true)
}

func (e *Evaluator) structureCaseProperty(element Element, property string) (Value, bool, bool) {
	switch property {
	case "actorParameter":
		return e.structureEffectiveMembershipMembers(element, "ActorMembership", "ownedActorParameter", "PartUsage", false)
	case "subjectParameter":
		return e.structureEffectiveMembershipMembers(element, "SubjectMembership", "ownedSubjectParameter", "Usage", true)
	case "objectiveRequirement":
		return e.structureEffectiveMembershipMembers(element, "ObjectiveMembership", "ownedObjectiveRequirement", "RequirementUsage", true)
	}
	return Value{}, false, false
}

func (e *Evaluator) structureRequirementProperty(element Element, property string) (Value, bool, bool) {
	switch property {
	case "actorParameter":
		return e.structureEffectiveMembershipMembers(element, "ActorMembership", "ownedActorParameter", "PartUsage", false)
	case "stakeholderParameter":
		return e.structureEffectiveMembershipMembers(element, "StakeholderMembership", "ownedStakeholderParameter", "PartUsage", false)
	case "subjectParameter":
		return e.structureEffectiveMembershipMembers(element, "SubjectMembership", "ownedSubjectParameter", "Usage", true)
	case "framedConcern":
		return e.structureOwnedMembershipMembers(element, "FramedConcernMembership", "ownedConcern", "ConcernUsage", false)
	case "assumedConstraint":
		return e.structureRequirementConstraints(element, "assumption")
	case "requiredConstraint":
		return e.structureRequirementConstraints(element, "requirement")
	case "text":
		return e.structureDocumentationText(element)
	}
	return Value{}, false, false
}

func (e *Evaluator) structureEffectiveMembershipMembers(element Element, membershipClass, property, expected string, single bool) (Value, bool, bool) {
	memberships, handled, ok := e.structureEffectiveFeatureMemberships(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			return Value{}, true, false
		}
		membership := MembershipElement(value.Membership)
		matches, known := e.isMetaclass(membership, membershipClass)
		if !known {
			return Value{}, true, false
		}
		if !matches {
			continue
		}
		related, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(related, expected)
		if !ok {
			return Value{}, true, false
		}
		values = append(values, filtered...)
	}
	return structurePropertyCardinality(values, single)
}

func (e *Evaluator) structureOwnedMembershipMembers(element Element, membershipClass, property, expected string, single bool) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, membershipClass, true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, value := range memberships.Values {
		membership := MembershipElement(value.Membership)
		related, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(related, expected)
		if !ok {
			return Value{}, true, false
		}
		values = append(values, filtered...)
	}
	return structurePropertyCardinality(values, single)
}

func (e *Evaluator) structureRequirementConstraints(element Element, kind string) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "RequirementConstraintMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, value := range memberships.Values {
		membership := MembershipElement(value.Membership)
		memberKind, ok := e.options.Structure.Attribute(membership, "kind")
		if !ok || memberKind.Kind != StringValue {
			return Value{}, true, false
		}
		if memberKind.String != kind {
			continue
		}
		related, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(related, "ConstraintUsage")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, filtered...)
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureDocumentationText(element Element) (Value, bool, bool) {
	documentation, handled, ok := e.structureOwnedFilter(element, "Documentation", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, value := range documentation.Values {
		if value.Kind != ElementValue {
			return Value{}, true, false
		}
		body, ok := e.options.Structure.Attribute(value.Element, "body")
		if !ok {
			return Value{}, true, false
		}
		if body.Kind == SequenceValue {
			for _, item := range body.Values {
				if item.Kind != StringValue {
					return Value{}, true, false
				}
				values = append(values, item)
			}
		} else if body.Kind == StringValue {
			values = append(values, body)
		} else {
			return Value{}, true, false
		}
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureStateProperty(element Element, property string) (Value, bool, bool) {
	if property == "state" {
		return e.structureFilteredEffectiveFeatures(element, "StateUsage")
	}
	expectedKind := map[string]string{
		"doAction":    "do",
		"entryAction": "entry",
		"exitAction":  "exit",
	}[property]
	if expectedKind == "" {
		return Value{}, false, false
	}
	memberships, handled, ok := e.structureOwnedFilter(element, "StateSubactionMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var actions []Value
	for _, value := range memberships.Values {
		membership := MembershipElement(value.Membership)
		kind, ok := e.options.Structure.Attribute(membership, "kind")
		if !ok || kind.Kind != StringValue {
			return Value{}, true, false
		}
		if kind.String != expectedKind {
			continue
		}
		related, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(related, "ActionUsage")
		if !ok {
			return Value{}, true, false
		}
		actions = append(actions, filtered...)
	}
	return structurePropertyCardinality(actions, true)
}

func (e *Evaluator) structureTransitionProperty(element Element, property string) (Value, bool, bool) {
	switch property {
	case "source", "target":
		return e.structureTransitionEndpoint(element, property)
	case "succession":
		return e.structureOwnedRelatedType(element, "Succession", true)
	case "triggerAction":
		return e.structureTransitionActions(element, "trigger", "AcceptActionUsage")
	case "effectAction":
		return e.structureTransitionActions(element, "effect", "ActionUsage")
	case "guardExpression":
		return e.structureTypedRelatedValues(element, property, "Expression", false)
	}
	return Value{}, false, false
}

func (e *Evaluator) structureTransitionEndpoint(element Element, endpoint string) (Value, bool, bool) {
	owner, present, known := e.structureOwnerElement(element)
	if !known || !present || owner.Symbol == nil || e.model == nil {
		return Value{}, true, false
	}
	for _, succession := range e.model.ActionSuccessions(owner.Symbol) {
		if succession.Decl != element.Node {
			continue
		}
		end := succession.Source
		if endpoint == "target" {
			end = succession.Target
		}
		target := end.Symbol
		if target == nil && end.Node != nil && e.resolver != nil {
			target, _ = e.resolver.ResolveTarget(owner.Symbol.Scope, end.Node)
		}
		if target == nil {
			return Value{}, true, false
		}
		return elementValue(ElementOf(target)), true, true
	}
	return Value{}, true, false
}

func (e *Evaluator) structureOwnedRelatedType(element Element, expected string, single bool) (Value, bool, bool) {
	members, handled, ok := e.structureOwnedMembers(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var matches []Value
	for _, member := range members.Values {
		if member.Kind != ElementValue {
			return Value{}, true, false
		}
		isExpected, known := e.isMetaclass(member.Element, expected)
		if !known {
			return Value{}, true, false
		}
		if isExpected {
			matches = append(matches, member)
		}
	}
	return structurePropertyCardinality(matches, single)
}

func (e *Evaluator) structureTransitionActions(element Element, kind, expected string) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "TransitionFeatureMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var actions []Value
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			return Value{}, true, false
		}
		membership := MembershipElement(value.Membership)
		memberKind, ok := e.options.Structure.Attribute(membership, "kind")
		if !ok || memberKind.Kind != StringValue {
			return Value{}, true, false
		}
		if memberKind.String != kind {
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(members, expected)
		if !ok {
			return Value{}, true, false
		}
		actions = append(actions, filtered...)
	}
	return sequence(actions...), true, true
}

func (e *Evaluator) structureFlowProperty(element Element, property string) (Value, bool, bool) {
	switch property {
	case "flowEnd":
		features, handled, ok := e.structureOwnedFeatures(element)
		if !handled || !ok {
			return Value{}, true, false
		}
		return e.structureFilterValuesAsSequence(features.Values, "FlowEnd")
	case "payloadFeature":
		features, handled, ok := e.structureOwnedFeatures(element)
		if !handled || !ok {
			return Value{}, true, false
		}
		return e.structureFilterValuesAsSequence(features.Values, "PayloadFeature")
	case "payloadType":
		features, handled, ok := e.structureFlowProperty(element, "payloadFeature")
		if !handled || !ok {
			return Value{}, true, false
		}
		var types []Value
		for _, feature := range features.Values {
			if feature.Kind != ElementValue {
				return Value{}, true, false
			}
			featureTypes, handled, ok := e.structureFeatureTypes(feature.Element)
			if !handled || !ok {
				return Value{}, true, false
			}
			filtered, ok := e.structureFilterValues(featureTypes.Values, "Classifier")
			if !ok {
				return Value{}, true, false
			}
			types = append(types, filtered...)
		}
		return sequence(uniqueValueSequence(types).Values...), true, true
	case "sourceOutputFeature":
		return e.structureFlowAttachment(element, 0)
	case "targetInputFeature":
		return e.structureFlowAttachment(element, 1)
	case "interaction":
		return e.structureTypedFeatureTypes(element, "Interaction", false)
	}
	return Value{}, false, false
}

func (e *Evaluator) structureFlowAttachment(element Element, index int) (Value, bool, bool) {
	if element.Symbol == nil || element.Symbol.Scope == nil || e.model == nil || e.resolver == nil {
		return Value{}, true, false
	}
	attachments := e.model.FlowEndAttachments(element.Symbol)
	if index >= len(attachments) {
		return nullValue(), true, true
	}
	ends, handled, ok := e.structureFlowProperty(element, "flowEnd")
	if !handled || !ok || len(ends.Values) != len(attachments) {
		return Value{}, true, false
	}
	target, ok := e.resolver.ResolveTarget(element.Symbol.Scope, attachments[index].Attachment)
	if !ok || target == nil {
		return Value{}, true, false
	}
	return elementValue(ElementOf(target)), true, true
}

func (e *Evaluator) structureFilterValuesAsSequence(values []Value, expected string) (Value, bool, bool) {
	filtered, ok := e.structureFilterValues(values, expected)
	if !ok {
		return Value{}, true, false
	}
	return sequence(filtered...), true, true
}

func structureRange(class, property string) (string, bool) {
	metaclasses := map[string]string{
		"ownedAnnotation":          "Annotation",
		"ownedConjugator":          "Conjugation",
		"ownedCrossSubsetting":     "CrossSubsetting",
		"ownedDifferencing":        "Differencing",
		"ownedDisjoining":          "Disjoining",
		"ownedFeatureChaining":     "FeatureChaining",
		"ownedFeatureInverting":    "FeatureInverting",
		"ownedFeatureMembership":   "FeatureMembership",
		"ownedImport":              "Import",
		"ownedIntersecting":        "Intersecting",
		"ownedMembership":          "Membership",
		"ownedRedefinition":        "Redefinition",
		"ownedReferenceSubsetting": "ReferenceSubsetting",
		"ownedSpecialization":      "Specialization",
		"ownedSubclassification":   "Subclassification",
		"ownedSubsetting":          "Subsetting",
		"ownedTypeFeaturing":       "TypeFeaturing",
		"ownedTyping":              "FeatureTyping",
		"ownedUnioning":            "Unioning",
	}
	metaclass, ok := metaclasses[property]
	return metaclass, ok
}

func (e *Evaluator) structureAttribute(element Element, property string) (Value, bool, bool) {
	value, ok := e.options.Structure.Attribute(element, property)
	if !ok {
		return Value{}, true, false
	}
	return value, true, true
}

func (e *Evaluator) semanticBooleanAttribute(element Element, property string) (Value, bool, bool) {
	if element.Symbol == nil || e.model == nil {
		return Value{}, true, false
	}
	var value bool
	switch property {
	case "isVariable":
		value = e.model.FeatureIsVariable(element.Symbol)
	case "isReference":
		value = semantics.UsageIsReferential(element.Symbol)
	case "mayTimeVary":
		value = e.model.UsageMayTimeVary(element.Symbol)
	default:
		reflective, ok := e.model.ReflectiveFeatureValue(element.Symbol, property)
		if !ok || reflective.Kind != symbols.FilterValueBool {
			return Value{}, true, false
		}
		value = reflective.Bool
	}
	return booleanValue(value), true, true
}

func (e *Evaluator) structureName(element Element, property string) (Value, bool, bool) {
	if element.Symbol != nil && !element.IsMembership {
		switch property {
		case "name":
			if name := e.model.EffectiveNameOf(element.Symbol); name != "" {
				return stringValue(name), true, true
			}
		case "shortName":
			if name := e.model.EffectiveShortNameOf(element.Symbol); name != "" {
				return stringValue(name), true, true
			}
		}
		return nullValue(), true, true
	}
	switch property {
	case "name":
		property = "declaredName"
	case "shortName":
		property = "declaredShortName"
	}
	value, ok := e.options.Structure.Attribute(element, property)
	if !ok {
		return Value{}, true, false
	}
	if value.Kind == StringValue && value.String != "" {
		return value, true, true
	}
	return nullValue(), true, true
}

func (e *Evaluator) structureAnnotatedElements(element Element) (Value, bool, bool) {
	annotations, handled, ok := e.structureOwnedFilter(element, "Annotation", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(annotations.Values))
	for _, annotation := range annotations.Values {
		if annotation.Kind != ElementValue {
			return Value{}, true, false
		}
		targets, ok := e.relationshipEndpoints(annotation.Element, "annotatedElement")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, targets...)
	}
	return uniqueValueSequence(values), true, true
}

func (e *Evaluator) structureQualifiedName(element Element, seen map[ElementKey]bool) (Value, bool, bool) {
	key := element.Key()
	if seen[key] {
		return Value{}, true, false
	}
	seen[key] = true
	name, handled, ok := e.structureName(element, "name")
	if !handled || !ok {
		return Value{}, true, false
	}
	if name.Kind != StringValue || name.String == "" {
		return nullValue(), true, true
	}
	namespace, handled, ok := e.structureOwningNamespace(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if namespace.Kind != ElementValue {
		return nullValue(), true, true
	}
	owner := namespace.Element
	members, handled, ok := e.structureEarlierOwnedMembers(owner, element)
	if !handled || !ok {
		return Value{}, true, false
	}
	for _, member := range members {
		memberName, handled, ok := e.structureName(member, "name")
		if !handled || !ok {
			return Value{}, true, false
		}
		if memberName.Kind == StringValue && memberName.String == name.String {
			return nullValue(), true, true
		}
	}
	parentNamespace, handled, ok := e.structureOwningNamespace(owner)
	if !handled || !ok {
		return Value{}, true, false
	}
	escapedName := identity.EscapeName(name.String)
	if parentNamespace.Kind != ElementValue {
		return stringValue(escapedName), true, true
	}
	parent, handled, ok := e.structureQualifiedName(owner, seen)
	if !handled || !ok {
		return Value{}, true, false
	}
	if parent.Kind == NullValue {
		return nullValue(), true, true
	}
	return stringValue(parent.String + "::" + escapedName), true, true
}

func (e *Evaluator) structureEarlierOwnedMembers(owner, element Element) ([]Element, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(owner)
	if !ok {
		return nil, true, false
	}
	var earlier []Element
	for _, relationship := range relationships {
		if relationship.Key() == element.Key() {
			return earlier, true, true
		}
		isMembership, known := e.isMetaclass(relationship, "Membership")
		if !known {
			return nil, true, false
		}
		if !isMembership {
			continue
		}
		members, ok := e.structureMembershipMembers(relationship)
		if !ok {
			return nil, true, false
		}
		for _, member := range members {
			if !element.IsMembership && member.Key() == element.Key() {
				return earlier, true, true
			}
			earlier = append(earlier, member)
		}
	}
	return earlier, true, true
}

func (e *Evaluator) structureOwnerElement(element Element) (Element, bool, bool) {
	owner, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Element{}, false, false
	}
	if present {
		owner, ownerPresent, ownerKnown := e.options.Structure.OwningRelatedElement(owner)
		return owner, ownerPresent, ownerKnown
	}
	return e.options.Structure.OwningRelatedElement(element)
}

func (e *Evaluator) structureOwner(element Element) (Value, bool, bool) {
	owner, present, known := e.structureOwnerElement(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	return elementValue(owner), true, true
}

func (e *Evaluator) structureOwningTypeOf(element Element, expected string) (Value, bool, bool) {
	owningType, handled, ok := e.structureOwningType(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if owningType.Kind != ElementValue {
		return nullValue(), true, true
	}
	matches, known := e.isMetaclass(owningType.Element, expected)
	if !known {
		return Value{}, true, false
	}
	if !matches {
		return nullValue(), true, true
	}
	return owningType, true, true
}

func (e *Evaluator) structureOwningMembership(element Element) (Value, bool, bool) {
	if element.IsMembership {
		return nullValue(), true, true
	}
	relationship, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	isMembership, known := e.isMetaclass(relationship, "Membership")
	if !known {
		return Value{}, true, false
	}
	if !isMembership || !relationship.IsMembership {
		return nullValue(), true, true
	}
	return membershipValue(relationship.Membership), true, true
}

func (e *Evaluator) structureOwningNamespace(element Element) (Value, bool, bool) {
	owner, present, known := e.structureOwnerElement(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	isNamespace, known := e.isMetaclass(owner, "Namespace")
	if !known {
		return Value{}, true, false
	}
	if !isNamespace {
		return nullValue(), true, true
	}
	return elementValue(owner), true, true
}

func (e *Evaluator) structureOptional(element Element) (Value, bool, bool) {
	value, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	return elementValue(value), true, true
}

func (e *Evaluator) structureOwningAnnotatingRelationship(element Element) (Value, bool, bool) {
	value, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	matches, known := e.isMetaclass(value, "Annotation")
	if !known {
		return Value{}, true, false
	}
	if !matches {
		return nullValue(), true, true
	}
	return elementValue(value), true, true
}

func (e *Evaluator) structureOwnedRelationshipValues(element Element) (Value, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(relationships))
	for _, relationship := range relationships {
		values = append(values, elementValue(relationship))
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureOwnedElements(element Element) (Value, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		return Value{}, true, false
	}
	var values []Element
	for _, relationship := range relationships {
		related, ok := e.options.Structure.OwnedRelatedElements(relationship)
		if !ok {
			return Value{}, true, false
		}
		values = append(values, related...)
	}
	return structureSequence(values), true, true
}

func (e *Evaluator) structureOwnedFilter(element Element, expected string, memberships bool) (Value, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		return Value{}, true, false
	}
	values := make([]Element, 0, len(relationships))
	for _, relationship := range relationships {
		matches, known := e.isMetaclass(relationship, expected)
		if !known {
			return Value{}, true, false
		}
		if matches {
			values = append(values, relationship)
		}
	}
	if memberships {
		sequence, ok := structureMembershipSequence(values)
		return sequence, true, ok
	}
	return structureSequence(values), true, true
}

func (e *Evaluator) structureOwnedFeatures(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "FeatureMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var features []Value
	for _, membership := range memberships.Values {
		handle := Element{Membership: membership.Membership, IsMembership: true}
		members, ok := e.structureMembershipMembers(handle)
		if !ok {
			return Value{}, true, false
		}
		for _, member := range members {
			if member.IsMembership {
				continue
			}
			isFeature, known := e.isMetaclass(member, "Feature")
			if !known {
				return Value{}, true, false
			}
			if isFeature {
				features = append(features, elementValue(member))
			}
		}
	}
	return sequence(features...), true, true
}

func (e *Evaluator) structureOwnedMembers(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var members []Element
	for _, membership := range memberships.Values {
		handle := Element{Membership: membership.Membership, IsMembership: true}
		values, ok := e.structureMembershipMembers(handle)
		if !ok {
			return Value{}, true, false
		}
		members = append(members, values...)
	}
	return structureSequence(members), true, true
}

func (e *Evaluator) structureEffectiveFeatures(element Element) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFeatures(element)
	if !handled || !ok || element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		return Value{}, true, false
	}
	inherited := e.inheritedSymbols(element.Symbol, all)
	values := append(slicesClone(owned.Values), symbolSequence(inherited).Values...)
	return sequence(values...), true, true
}

func (e *Evaluator) structureDirectedFeatures(element Element, property string) (Value, bool, bool) {
	ownedFeatures, ownedHandled, ownedOK := e.structureOwnedFeatures(element)
	if !ownedHandled || !ownedOK {
		return Value{}, true, false
	}
	owned := make(map[ElementKey]bool, len(ownedFeatures.Values))
	for _, feature := range ownedFeatures.Values {
		if feature.Kind != ElementValue {
			return Value{}, true, false
		}
		owned[feature.Element.Key()] = true
	}
	features, handled, ok := e.structureEffectiveFeatures(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var directed []Value
	for _, feature := range features.Values {
		if feature.Kind != ElementValue {
			return Value{}, true, false
		}
		if property == "directedFeature" {
			if owned[feature.Element.Key()] {
				direction, ok := e.options.Structure.Attribute(feature.Element, "direction")
				if !ok {
					return Value{}, true, false
				}
				if !hasDirection(direction) {
					continue
				}
			} else {
				if element.Symbol == nil || feature.Element.Symbol == nil {
					return Value{}, true, false
				}
				if e.model.EffectiveDirection(element.Symbol, feature.Element.Symbol) == ast.DirNone {
					continue
				}
			}
		} else {
			if element.Symbol == nil || feature.Element.Symbol == nil {
				return Value{}, true, false
			}
			direction := e.model.EffectiveDirection(element.Symbol, feature.Element.Symbol)
			if property == "input" && direction != ast.DirIn && direction != ast.DirInOut {
				continue
			}
			if property == "output" && direction != ast.DirOut && direction != ast.DirInOut {
				continue
			}
		}
		directed = append(directed, feature)
	}
	return sequence(directed...), true, true
}

func hasDirection(value Value) bool {
	switch value.Kind {
	case EnumValue:
		return value.Enum != "" && value.Enum != ast.DirNone.String()
	case StringValue:
		return value.String != "" && value.String != ast.DirNone.String()
	default:
		return false
	}
}

func (e *Evaluator) structureDirectedUsages(element Element) (Value, bool, bool) {
	directed, handled, ok := e.structureDirectedFeatures(element, "directedFeature")
	if !handled || !ok {
		return Value{}, true, false
	}
	var usages []Value
	for _, feature := range directed.Values {
		if feature.Kind != ElementValue {
			return Value{}, true, false
		}
		matches, known := e.isMetaclass(feature.Element, "Usage")
		if !known {
			return Value{}, true, false
		}
		if matches {
			usages = append(usages, feature)
		}
	}
	return sequence(usages...), true, true
}

func (e *Evaluator) structureInheritedFeatures(element Element) (Value, bool, bool) {
	if element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		return Value{}, true, false
	}
	return symbolSequence(e.inheritedSymbols(element.Symbol, all)), true, true
}

func (e *Evaluator) structureEffectiveFeatureMemberships(element Element) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFilter(element, "FeatureMembership", true)
	if !handled || !ok || element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		return Value{}, true, false
	}
	inherited, ok := e.structureFeatureMemberships(e.inheritedSymbols(element.Symbol, all))
	if !ok {
		return Value{}, true, false
	}
	return sequence(append(slicesClone(owned.Values), membershipSequence(inherited).Values...)...), true, true
}

func (e *Evaluator) structureFeatureMemberships(members []*symbols.Symbol) ([]Membership, bool) {
	all := memberships(members)
	features := make([]Membership, 0, len(all))
	for _, membership := range all {
		isFeatureMembership, known := e.isMetaclass(MembershipElement(membership), "FeatureMembership")
		if !known {
			return nil, false
		}
		if isFeatureMembership {
			features = append(features, membership)
		}
	}
	return features, true
}

func (e *Evaluator) structureInheritedMemberships(element Element) (Value, bool, bool) {
	if element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "member")
	if !ok {
		return Value{}, true, false
	}
	return membershipSequence(memberships(e.inheritedSymbols(element.Symbol, all))), true, true
}

func (e *Evaluator) structureVariantProperty(element Element, property string) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	if element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "member")
	if !ok {
		return Value{}, true, false
	}
	inherited := membershipSequence(memberships(e.inheritedSymbols(element.Symbol, all)))
	effective := append(slicesClone(owned.Values), inherited.Values...)
	var variants []Value
	for _, value := range effective {
		if value.Kind != MembershipValue {
			return Value{}, true, false
		}
		membership := Element{Membership: value.Membership, IsMembership: true}
		isVariant, known := e.isMetaclass(membership, "VariantMembership")
		if !known {
			return Value{}, true, false
		}
		if !isVariant {
			continue
		}
		if property == "variantMembership" {
			variants = append(variants, value)
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(structureSequence(members).Values, "Usage")
		if !ok {
			return Value{}, true, false
		}
		variants = append(variants, filtered...)
	}
	return sequence(variants...), true, true
}

func (e *Evaluator) structureOwnedEndFeatures(element Element) (Value, bool, bool) {
	features, handled, ok := e.structureOwnedFeatures(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var ends []Value
	for _, value := range features.Values {
		isEnd, ok := e.options.Structure.Attribute(value.Element, "isEnd")
		if !ok {
			return Value{}, true, false
		}
		if isEnd.Kind == BooleanValue && isEnd.Boolean {
			ends = append(ends, value)
		}
	}
	return sequence(ends...), true, true
}

func (e *Evaluator) structureEffectiveEndFeatures(element Element) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedEndFeatures(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	inherited, handled, ok := e.structureInheritedFeatures(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	ends := slicesClone(owned.Values)
	for _, feature := range inherited.Values {
		if feature.Kind != ElementValue || feature.Element.Symbol == nil {
			return Value{}, true, false
		}
		isEnd, ok := e.model.ReflectiveFeatureValue(feature.Element.Symbol, "isEnd")
		if !ok || isEnd.Kind != symbols.FilterValueBool {
			return Value{}, true, false
		}
		if isEnd.Bool {
			ends = append(ends, feature)
		}
	}
	return sequence(ends...), true, true
}

func (e *Evaluator) structureFeatureTypes(element Element) (Value, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0)
	for _, relationship := range relationships {
		matches, known := e.isMetaclass(relationship, "FeatureTyping")
		if !known {
			return Value{}, true, false
		}
		if !matches {
			continue
		}
		targets, ok := e.relationshipEndpoints(relationship, "type")
		if !ok {
			return Value{}, true, false
		}
		targets, ok = e.structureFilterValues(targets, "Type")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, targets...)
	}
	if element.Symbol == nil || !e.completeSupertypes(element.Symbol) {
		return Value{}, true, false
	}
	inherited, ok := e.featureTypes(element.Symbol)
	if !ok {
		return Value{}, true, false
	}
	for _, target := range inherited {
		if target == nil {
			return Value{}, true, false
		}
		value := elementValue(ElementOf(target))
		alreadyOwned := false
		for _, owned := range values {
			if owned.Kind == ElementValue && owned.Element.Key() == value.Element.Key() {
				alreadyOwned = true
				break
			}
		}
		if !alreadyOwned {
			values = append(values, value)
		}
	}
	return uniqueValueSequence(values), true, true
}

func (e *Evaluator) structureTypedDefinition(element Element, expected string, many bool) (Value, bool, bool) {
	types, handled, ok := e.structureFeatureTypes(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var definitions []Value
	for _, typ := range types.Values {
		matches, known := e.isMetaclass(typ.Element, expected)
		if !known {
			return Value{}, true, false
		}
		if matches {
			definitions = append(definitions, typ)
		}
	}
	if many {
		return sequence(definitions...), true, true
	}
	if len(definitions) > 1 {
		return Value{}, true, false
	}
	if len(definitions) == 0 {
		return nullValue(), true, true
	}
	return definitions[0], true, true
}

func (e *Evaluator) structureTypeOperands(element Element, property string) (Value, bool, bool) {
	relationshipClass := map[string]string{
		"differencingType": "Differencing",
		"intersectingType": "Intersecting",
		"unioningType":     "Unioning",
	}[property]
	relationships, handled, ok := e.structureOwnedFilter(element, relationshipClass, false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, relationship := range relationships.Values {
		operands, ok := e.relationshipEndpoints(relationship.Element, "target")
		if !ok {
			return Value{}, true, false
		}
		operands, ok = e.structureFilterValues(operands, "Type")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, operands...)
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureIsConjugated(element Element) (Value, bool, bool) {
	conjugators, handled, ok := e.structureOwnedFilter(element, "Conjugation", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	return booleanValue(len(conjugators.Values) != 0), true, true
}

func (e *Evaluator) structureOwningFeatureMembership(element Element) (Value, bool, bool) {
	relationship, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	isFeatureMembership, known := e.isMetaclass(relationship, "FeatureMembership")
	if !known {
		return Value{}, true, false
	}
	if !isFeatureMembership || !relationship.IsMembership {
		return nullValue(), true, true
	}
	return membershipValue(relationship.Membership), true, true
}

func (e *Evaluator) structureOwningType(element Element) (Value, bool, bool) {
	membership, handled, ok := e.structureOwningFeatureMembership(element)
	if !handled || !ok || membership.Kind == NullValue {
		return nullValue(), true, ok
	}
	handle := Element{Membership: membership.Membership, IsMembership: true}
	owner, present, known := e.options.Structure.OwningRelatedElement(handle)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	isType, known := e.isMetaclass(owner, "Type")
	if !known {
		return Value{}, true, false
	}
	if !isType {
		return nullValue(), true, true
	}
	return elementValue(owner), true, true
}

func (e *Evaluator) structureEndOwningType(element Element) (Value, bool, bool) {
	isEnd, ok := e.options.Structure.Attribute(element, "isEnd")
	if !ok {
		return Value{}, true, false
	}
	if isEnd.Kind != BooleanValue || !isEnd.Boolean {
		return nullValue(), true, true
	}
	return e.structureOwningType(element)
}

func (e *Evaluator) structureFeatureTarget(element Element) (Value, bool, bool) {
	relationships, handled, ok := e.structureOwnedFilter(element, "ReferenceSubsetting", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var targets []Value
	for _, relationship := range relationships.Values {
		endpoints, ok := e.relationshipEndpoints(relationship.Element, "referencedFeature")
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(endpoints, "Feature")
		if !ok {
			return Value{}, true, false
		}
		targets = append(targets, filtered...)
	}
	return structurePropertyCardinality(targets, true)
}

func (e *Evaluator) structureFeaturingTypes(element Element, seen map[ElementKey]bool) (Value, bool, bool) {
	key := element.Key()
	if seen[key] {
		return Value{}, true, false
	}
	seen[key] = true
	relations, handled, ok := e.structureOwnedFilter(element, "TypeFeaturing", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0)
	for _, relation := range relations.Values {
		types, ok := e.relationshipEndpoints(relation.Element, "featuringType")
		if !ok {
			return Value{}, true, false
		}
		types, ok = e.structureFilterValues(types, "Type")
		if !ok {
			return Value{}, true, false
		}
		values = append(values, types...)
	}
	owningMembership, handled, ok := e.structureOwningFeatureMembership(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if owningMembership.Kind == MembershipValue {
		variable, known := e.options.Structure.Attribute(element, "isVariable")
		if !known || variable.Kind != BooleanValue {
			return Value{}, true, false
		}
		if variable.Boolean {
			return Value{}, true, false
		}
		owningType, handled, ok := e.structureOwningType(element)
		if !handled || !ok {
			return Value{}, true, false
		}
		if owningType.Kind == ElementValue {
			values = append(values, owningType)
		}
	}
	chains, handled, ok := e.structureOwnedFilter(element, "FeatureChaining", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	if len(chains.Values) > 0 {
		endpoints, ok := e.relationshipEndpoints(chains.Values[0].Element, "chainingFeature")
		if !ok || len(endpoints) != 1 || endpoints[0].Kind != ElementValue {
			return Value{}, true, false
		}
		features, handled, ok := e.structureFeaturingTypes(endpoints[0].Element, seen)
		if !handled || !ok {
			return Value{}, true, false
		}
		values = append(values, features.Values...)
	}
	return uniqueValueSequence(values), true, true
}

func (e *Evaluator) structureMemberName(element Element, memberProperty, nameProperty string) (Value, bool, bool) {
	if memberProperty != "" {
		if value, ok := e.options.Structure.Attribute(element, memberProperty); ok &&
			value.Kind == StringValue && value.String != "" {
			return value, true, true
		}
	}
	members, ok := e.structureMembershipMembers(element)
	if !ok {
		return Value{}, true, false
	}
	if len(members) == 0 {
		return nullValue(), true, true
	}
	if len(members) != 1 {
		return Value{}, true, false
	}
	name, ok := e.options.Structure.Attribute(members[0], nameProperty)
	if !ok {
		return Value{}, true, false
	}
	if name.Kind == StringValue && name.String != "" {
		return name, true, true
	}
	return nullValue(), true, true
}

func (e *Evaluator) structureNamespaceMemberships(element Element) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return Value{}, handled, false
	}
	if element.Symbol == nil || !e.completeMembers(element.Symbol) {
		return Value{}, true, false
	}
	members, ok := e.model.ReflectiveElements(element.Symbol, "member")
	if !ok {
		return Value{}, true, false
	}
	inherited := memberships(e.inheritedSymbols(element.Symbol, members))
	imported, handled, ok := e.structureImportedMemberships(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	return sequence(append(append(slicesClone(owned.Values), membershipSequence(inherited).Values...), imported.Values...)...), true, true
}

func (e *Evaluator) structureImportedMemberships(element Element) (Value, bool, bool) {
	imports, handled, ok := e.structureOwnedFilter(element, "Import", false)
	if !handled || !ok {
		return Value{}, handled, false
	}
	if len(imports.Values) == 0 {
		return sequence(), true, true
	}
	if element.Symbol == nil || element.Symbol.Scope == nil || e.resolver == nil {
		return Value{}, true, false
	}
	memberships := make([]Element, 0)
	seen := make(map[MembershipKey]bool)
	for _, imported := range imports.Values {
		imp, ok := imported.Element.Node.(*ast.Import)
		if !ok || imp == nil {
			return Value{}, true, false
		}
		target, ok := e.resolver.ImportTarget(element.Symbol.Scope, imp)
		if !ok || target == nil || !e.importTreeComplete(target, imp, make(map[*symbols.Scope]bool)) {
			return Value{}, true, false
		}
		for _, symbol := range e.resolver.ImportedElements(element.Symbol.Scope, imp) {
			if symbol == nil {
				return Value{}, true, false
			}
			if symbol == target && (imp.Kind == ast.ImportNamespace || imp.IsRecursive) {
				continue
			}
			membership := ElementOf(symbol).Membership
			if !hasMembership(membership) {
				return Value{}, true, false
			}
			if seen[membership.Key()] {
				continue
			}
			seen[membership.Key()] = true
			memberships = append(memberships, MembershipElement(membership))
		}
	}
	value, ok := structureMembershipSequence(memberships)
	return value, true, ok
}

func (e *Evaluator) structureNamespaceMembers(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureNamespaceMemberships(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, membership := range memberships.Values {
		handle := Element{Membership: membership.Membership, IsMembership: true}
		members, ok := e.structureMembershipMembers(handle)
		if !ok {
			return Value{}, true, false
		}
		for _, member := range members {
			values = append(values, elementValue(member))
		}
	}
	return uniqueValueSequence(values), true, true
}

func (e *Evaluator) structureFilteredOwnedFeatures(element Element, property, _ string) (Value, bool, bool) {
	rangeClass, ok := e.ownedUsageMetaclass(property)
	if !ok {
		return Value{}, true, false
	}
	members, handled, ok := e.structureOwnedMembers(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, member := range members.Values {
		var candidate Element
		switch member.Kind {
		case ElementValue:
			candidate = member.Element
		case MembershipValue:
			candidate = MembershipElement(member.Membership)
		default:
			return Value{}, true, false
		}
		matches, known := e.isMetaclass(candidate, rangeClass)
		if !known {
			return Value{}, true, false
		}
		if matches {
			values = append(values, member)
		}
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureFilteredEffectiveFeatures(element Element, expected string) (Value, bool, bool) {
	features, handled, ok := e.structureEffectiveFeatures(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var values []Value
	for _, feature := range features.Values {
		matches, known := e.isMetaclass(feature.Element, expected)
		if !known {
			return Value{}, true, false
		}
		if matches {
			values = append(values, feature)
		}
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureTypedFeatureTypes(element Element, expected string, single bool) (Value, bool, bool) {
	types, handled, ok := e.structureFeatureTypes(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(types.Values))
	for _, typ := range types.Values {
		matches, known := e.isMetaclass(typ.Element, expected)
		if !known {
			return Value{}, true, false
		}
		if matches {
			values = append(values, typ)
		}
	}
	return structurePropertyCardinality(values, single)
}

func (e *Evaluator) structureBehaviorParameters(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureEffectiveFeatureMemberships(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var parameters []Value
	for _, value := range memberships.Values {
		membership := Element{Membership: value.Membership, IsMembership: true}
		isParameter, known := e.isMetaclass(membership, "ParameterMembership")
		if !known {
			return Value{}, true, false
		}
		if !isParameter {
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			return Value{}, true, false
		}
		filtered, handled, ok := e.structureFilterRelated(members, "Feature", false)
		if !handled || !ok {
			return Value{}, true, false
		}
		parameters = append(parameters, filtered.Values...)
	}
	return sequence(parameters...), true, true
}

func (e *Evaluator) structureStepParameters(element Element) (Value, bool, bool) {
	behaviors, handled, ok := e.structureTypedFeatureTypes(element, "Behavior", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	var parameters []Value
	for _, behavior := range behaviors.Values {
		values, handled, ok := e.structureBehaviorParameters(behavior.Element)
		if !handled || !ok {
			return Value{}, true, false
		}
		parameters = append(parameters, values.Values...)
	}
	return sequence(parameters...), true, true
}

func (e *Evaluator) structureRelationshipProperty(element Element, class, name string) (Value, bool, bool) {
	switch name {
	case "ownedRelatedElement":
		related, ok := e.options.Structure.OwnedRelatedElements(element)
		if !ok {
			return Value{}, true, false
		}
		return structureSequence(related), true, true
	case "owningRelatedElement":
		owner, present, known := e.options.Structure.OwningRelatedElement(element)
		if !known {
			return Value{}, true, false
		}
		if !present {
			return nullValue(), true, true
		}
		return elementValue(owner), true, true
	case "relatedElement":
		source, sourceOK := e.relationshipEndpoints(element, "source")
		target, targetOK := e.relationshipEndpoints(element, "target")
		if !sourceOK || !targetOK {
			return Value{}, true, false
		}
		return uniqueValueSequence(append(source, target...)), true, true
	case "owningType":
		return e.structureRelationshipOwner(element, "Type")
	case "owningFeature":
		return e.structureRelationshipOwner(element, "Feature")
	case "importOwningNamespace":
		return e.structureRelationshipOwner(element, "Namespace")
	case "owningAnnotatedElement":
		endpoints, ok := e.relationshipEndpoints(element, "annotatedElement")
		if !ok {
			return Value{}, true, false
		}
		return structureRelationshipValues(endpoints, class, name), true, true
	case "documentedElement":
		return e.structureRelationshipOwner(element, "")
	case "owningClassifier":
		return e.structureRelationshipOwner(element, "Classifier")
	case "owningFeatureOfType":
		return e.structureRelationshipOwner(element, "Feature")
	case "source", "target", "special", "general", "specific", "typedFeature", "type",
		"subsettingFeature", "subsettedFeature", "redefiningFeature", "redefinedFeature",
		"client", "supplier", "importedElement",
		"membershipOwningNamespace", "memberElement", "ownedMemberElement",
		"annotatedElement", "annotation", "annotatingElement", "owningAnnotatingElement",
		"ownedAnnotatingElement",
		"featureWithValue", "value",
		"conjugatedType", "originalType", "typeDisjoined", "disjoiningType",
		"differencingType", "typeDifferenced", "intersectingType", "typeIntersected",
		"typeUnioned", "unioningType", "featureOfType", "featuringType",
		"crossingFeature", "crossedFeature", "featureChained", "chainingFeature",
		"featureInverted", "invertingFeature", "subclassifier", "superclassifier":
		values, ok := e.relationshipEndpoints(element, name)
		if !ok {
			return Value{}, true, false
		}
		if expected := relationshipEndpointRange(class, name); expected != "" {
			values, ok = e.structureFilterValues(values, expected)
			if !ok {
				return Value{}, true, false
			}
		}
		return structureRelationshipValues(values, class, name), true, true
	}
	return Value{}, false, false
}

func relationshipEndpointRange(class, property string) string {
	switch class {
	case "Specialization":
		if oneOf(property, "source", "target", "specific", "special", "general") {
			return "Type"
		}
	case "FeatureTyping":
		if oneOf(property, "source", "typedFeature") {
			return "Feature"
		}
		if oneOf(property, "target", "type") {
			return "Type"
		}
	case "Subsetting":
		if oneOf(property, "source", "subsettingFeature", "target", "subsettedFeature") {
			return "Feature"
		}
	case "Redefinition":
		if oneOf(property, "source", "redefiningFeature", "target", "redefinedFeature") {
			return "Feature"
		}
	case "Membership":
		if oneOf(property, "source", "membershipOwningNamespace") {
			return "Namespace"
		}
	case "Import":
		if oneOf(property, "source", "importOwningNamespace") {
			return "Namespace"
		}
	case "FeatureValue":
		if oneOf(property, "source", "featureWithValue") {
			return "Feature"
		}
		if oneOf(property, "target", "value") {
			return "Expression"
		}
	case "Annotation":
		if oneOf(property, "source", "annotatingElement", "owningAnnotatingElement", "ownedAnnotatingElement") {
			return "AnnotatingElement"
		}
		if oneOf(property, "target", "annotatedElement") {
			return "Element"
		}
	case "Conjugation":
		return "Type"
	case "Disjoining":
		return "Type"
	case "Differencing":
		return "Type"
	case "Intersecting":
		return "Type"
	case "Unioning":
		return "Type"
	case "TypeFeaturing":
		if oneOf(property, "source", "featureOfType", "owningFeatureOfType") {
			return "Feature"
		}
		if oneOf(property, "target", "featuringType") {
			return "Type"
		}
	case "Connector":
		if oneOf(property, "source", "sourceFeature", "target", "targetFeature", "relatedElement") {
			return "Feature"
		}
	case "CrossSubsetting":
		return "Feature"
	case "FeatureChaining":
		return "Feature"
	case "FeatureInverting":
		return "Feature"
	case "Subclassification":
		return "Classifier"
	}
	return ""
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func (e *Evaluator) structureRelationshipOwner(element Element, expected string) (Value, bool, bool) {
	owner, present, known := e.options.Structure.OwningRelatedElement(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	if expected != "" {
		matches, known := e.isMetaclass(owner, expected)
		if !known {
			return Value{}, true, false
		}
		if !matches {
			return nullValue(), true, true
		}
	}
	return elementValue(owner), true, true
}

func (e *Evaluator) relationshipEndpoints(element Element, property string) ([]Value, bool) {
	actual, known := e.options.Structure.Metaclass(element)
	if !known {
		return nil, false
	}
	endpoint := ""
	setEndpoint := func(name string) {
		endpoint = name
	}
	specializes := func(expected string) bool {
		return actual == expected || e.options.Structure.Specializes(actual, expected)
	}
	switch {
	case specializes("Subclassification"):
		if property == "source" || property == "subclassifier" {
			setEndpoint("subclassifier")
		} else if property == "target" || property == "superclassifier" {
			setEndpoint("superclassifier")
		}
	case specializes("FeatureTyping"):
		if property == "source" || property == "typedFeature" {
			setEndpoint("typedFeature")
		} else if property == "target" || property == "type" {
			setEndpoint("type")
		}
	case specializes("ReferenceSubsetting"):
		if property == "source" || property == "referencingFeature" {
			setEndpoint("referencingFeature")
		} else if property == "target" || property == "referencedFeature" {
			setEndpoint("referencedFeature")
		}
	case specializes("Redefinition"):
		if property == "source" || property == "redefiningFeature" {
			setEndpoint("redefiningFeature")
		} else if property == "target" || property == "redefinedFeature" {
			setEndpoint("redefinedFeature")
		}
	case specializes("TypeFeaturing"):
		if property == "source" || property == "featureOfType" {
			setEndpoint("featureOfType")
		} else if property == "target" || property == "featuringType" {
			setEndpoint("featuringType")
		}
	case specializes("FeatureChaining"):
		if property == "source" || property == "featureChained" {
			setEndpoint("featureChained")
		} else if property == "target" || property == "chainingFeature" {
			setEndpoint("chainingFeature")
		}
	case specializes("CrossSubsetting"):
		if property == "source" || property == "crossingFeature" {
			setEndpoint("crossingFeature")
		} else if property == "target" || property == "crossedFeature" {
			setEndpoint("crossedFeature")
		}
	case specializes("FeatureInverting"):
		if property == "source" || property == "featureInverted" {
			setEndpoint("featureInverted")
		} else if property == "target" || property == "invertingFeature" {
			setEndpoint("invertingFeature")
		}
	case specializes("Subsetting"):
		if property == "source" || property == "subsettingFeature" {
			setEndpoint("subsettingFeature")
		} else if property == "target" || property == "subsettedFeature" {
			setEndpoint("subsettedFeature")
		}
	case specializes("Specialization"):
		if property == "source" || property == "specific" || property == "special" {
			setEndpoint("specific")
		} else if property == "target" || property == "general" {
			setEndpoint("general")
		} else if property == "referencingFeature" {
			setEndpoint("referencingFeature")
		}
	case specializes("Membership"):
		if property == "source" || property == "membershipOwningNamespace" {
			setEndpoint("owning")
		} else if property == "target" || property == "memberElement" {
			setEndpoint("memberElement")
		}
	case specializes("Import"):
		if property == "source" || property == "importOwningNamespace" {
			setEndpoint("owning")
		} else if property == "target" || property == "importedElement" {
			setEndpoint("importedElement")
		}
	case specializes("FeatureValue"):
		if property == "source" || property == "featureWithValue" {
			setEndpoint("owning")
		} else if property == "target" || property == "value" {
			setEndpoint("value")
		}
	case specializes("Annotation"):
		if property == "source" || property == "annotatingElement" ||
			property == "owningAnnotatingElement" || property == "ownedAnnotatingElement" {
			setEndpoint("owning")
		} else if property == "target" || property == "annotatedElement" {
			setEndpoint("annotatedElement")
		}
	case specializes("Dependency"):
		if property == "source" || property == "client" {
			setEndpoint("client")
		} else if property == "target" || property == "supplier" {
			setEndpoint("supplier")
		}
	case specializes("Conjugation"):
		if property == "source" || property == "conjugatedType" {
			setEndpoint("conjugatedType")
		} else if property == "target" || property == "originalType" {
			setEndpoint("originalType")
		}
	case specializes("Disjoining"):
		if property == "source" || property == "typeDisjoined" {
			setEndpoint("typeDisjoined")
		} else if property == "target" || property == "disjoiningType" {
			setEndpoint("disjoiningType")
		}
	case specializes("Differencing"):
		if property == "source" || property == "typeDifferenced" {
			setEndpoint("typeDifferenced")
		} else if property == "target" || property == "differencingType" {
			setEndpoint("differencingType")
		}
	case specializes("Intersecting"):
		if property == "source" || property == "typeIntersected" {
			setEndpoint("typeIntersected")
		} else if property == "target" || property == "intersectingType" {
			setEndpoint("intersectingType")
		}
	case specializes("Unioning"):
		if property == "source" || property == "typeUnioned" {
			setEndpoint("typeUnioned")
		} else if property == "target" || property == "unioningType" {
			setEndpoint("unioningType")
		}
	case specializes("TypeFeaturing"):
		if property == "source" || property == "featureOfType" || property == "owningFeatureOfType" {
			setEndpoint("featureOfType")
		} else if property == "target" || property == "featuringType" {
			setEndpoint("featuringType")
		}
	case specializes("Connector"):
		if property == "source" || property == "sourceFeature" {
			setEndpoint("sourceFeature")
		} else if property == "target" || property == "targetFeature" {
			setEndpoint("targetFeature")
		}
	default:
		if property == "source" {
			setEndpoint("source")
		} else if property == "target" {
			setEndpoint("target")
		}
	}
	if property == "relatedElement" {
		source, sourceOK := e.relationshipEndpoints(element, "source")
		target, targetOK := e.relationshipEndpoints(element, "target")
		if !sourceOK || !targetOK {
			return nil, false
		}
		return uniqueValueSequence(append(source, target...)).Values, true
	}
	if endpoint == "" {
		return nil, false
	}
	if endpoint == "owning" {
		owner, present, known := e.options.Structure.OwningRelatedElement(element)
		if !known {
			return nil, false
		}
		if !present {
			return []Value{}, true
		}
		return []Value{elementValue(owner)}, true
	}
	if endpoint == "importedElement" {
		related, ok := e.options.Structure.RelatedElements(element, endpoint)
		if ok {
			values := make([]Value, 0, len(related))
			for _, target := range related {
				values = append(values, elementValue(target))
			}
			return values, true
		}
		if imp, ok := element.Node.(*ast.Import); ok && imp != nil && element.Container != nil && element.Container.Scope != nil && e.resolver != nil {
			target, ok := e.resolver.ImportTarget(element.Container.Scope, imp)
			if !ok || target == nil {
				return nil, false
			}
			return []Value{elementValue(ElementOf(target))}, true
		}
	}
	related, ok := e.options.Structure.RelatedElements(element, endpoint)
	if !ok && endpoint == "referencingFeature" {
		related, ok = e.options.Structure.RelatedElements(element, "subsettingFeature")
	}
	if !ok && endpoint == "value" {
		related, ok = e.options.Structure.OwnedRelatedElements(element)
	}
	if !ok {
		return nil, false
	}
	values := make([]Value, 0, len(related))
	for _, target := range related {
		values = append(values, elementValue(target))
	}
	return values, true
}

func (e *Evaluator) structureTypedRelatedValues(element Element, property, expected string, single bool) (Value, bool, bool) {
	values, ok := e.structureRelatedElements(element, property)
	if !ok {
		return Value{}, true, false
	}
	return e.structureFilterRelated(values, expected, single)
}

func (e *Evaluator) structureModelRelatedValues(element Element, property, expected string, single bool) (Value, bool, bool) {
	if element.Symbol == nil || e.model == nil {
		return Value{}, true, false
	}
	symbols, ok := e.model.ReflectiveElements(element.Symbol, property)
	if !ok {
		return Value{}, true, false
	}
	values := make([]Element, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol == nil {
			return Value{}, true, false
		}
		values = append(values, ElementOf(symbol))
	}
	return e.structureFilterRelated(values, expected, single)
}

func (e *Evaluator) structureResultFeatures(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "ReturnParameterMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var features []Value
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			return Value{}, true, false
		}
		members, ok := e.structureMembershipMembers(MembershipElement(value.Membership))
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(members, "Feature")
		if !ok {
			return Value{}, true, false
		}
		features = append(features, filtered...)
	}
	if len(features) == 0 && element.Symbol != nil {
		outputs, handled, ok := e.structureDirectedFeatures(element, "output")
		if !handled || !ok {
			return Value{}, true, false
		}
		features = outputs.Values
	}
	if len(features) == 0 {
		return Value{}, true, false
	}
	return structurePropertyCardinality(features, true)
}

func (e *Evaluator) structureMultiplicityBounds(element Element, property string) (Value, bool, bool) {
	if element.Container == nil || element.Aspect == "" {
		return Value{}, true, false
	}
	node := multiplicityOf(element.Container)
	multiplicity, ok := node.(*ast.Multiplicity)
	if !ok || multiplicity == nil {
		return Value{}, true, false
	}
	bounds := make([]struct {
		name string
		node ast.Node
	}, 0, 2)
	lower, upper := multiplicity.Lower, multiplicity.Upper
	if !multiplicity.IsRange {
		lower, upper = nil, multiplicity.Lower
	}
	switch property {
	case "lowerBound":
		if lower != nil {
			bounds = append(bounds, struct {
				name string
				node ast.Node
			}{name: "lowerBound", node: lower})
		}
	case "upperBound":
		if upper != nil {
			bounds = append(bounds, struct {
				name string
				node ast.Node
			}{name: "upperBound", node: upper})
		}
	case "bound":
		if lower != nil {
			bounds = append(bounds, struct {
				name string
				node ast.Node
			}{name: "lowerBound", node: lower})
		}
		if upper != nil {
			bounds = append(bounds, struct {
				name string
				node ast.Node
			}{name: "upperBound", node: upper})
		}
	default:
		return Value{}, true, false
	}
	aspect := strings.TrimSuffix(element.Aspect, "/multiplicity")
	aspect = strings.TrimSuffix(aspect, "multiplicity")
	values := make([]Value, 0, len(bounds))
	for _, bound := range bounds {
		name := bound.name
		if aspect != "" {
			name = aspect + "/" + name
		}
		values = append(values, elementValue(Element{
			Node: bound.node, Container: element.Container, Aspect: name,
		}))
	}
	if property == "bound" {
		return sequence(values...), true, true
	}
	return structurePropertyCardinality(values, true)
}

func (e *Evaluator) structureFilterRelated(elements []Element, expected string, single bool) (Value, bool, bool) {
	values := make([]Value, 0, len(elements))
	for _, element := range elements {
		matches, known := e.isMetaclass(element, expected)
		if !known {
			return Value{}, true, false
		}
		if matches {
			values = append(values, elementValue(element))
		}
	}
	return structurePropertyCardinality(values, single)
}

func (e *Evaluator) structureFilterValues(values []Value, expected string) ([]Value, bool) {
	filtered := make([]Value, 0, len(values))
	for _, value := range values {
		if value.Kind != ElementValue {
			return nil, false
		}
		matches, known := e.isMetaclass(value.Element, expected)
		if !known {
			return nil, false
		}
		if matches {
			filtered = append(filtered, value)
		}
	}
	return filtered, true
}

func structurePropertyCardinality(values []Value, single bool) (Value, bool, bool) {
	if !single {
		return sequence(values...), true, true
	}
	if len(values) > 1 {
		return Value{}, true, false
	}
	if len(values) == 0 {
		return nullValue(), true, true
	}
	return values[0], true, true
}

func (e *Evaluator) structureRelatedElements(element Element, properties ...string) ([]Element, bool) {
	known := false
	for _, property := range properties {
		values, ok := e.options.Structure.RelatedElements(element, property)
		if !ok {
			continue
		}
		known = true
		if len(values) > 0 {
			return values, true
		}
	}
	return nil, known
}

func (e *Evaluator) structureMembershipMembers(membership Element) ([]Element, bool) {
	values, ok := e.relationshipEndpoints(membership, "memberElement")
	if !ok {
		return nil, false
	}
	members := make([]Element, 0, len(values))
	for _, value := range values {
		if value.Kind != ElementValue {
			return nil, false
		}
		members = append(members, value.Element)
	}
	return members, true
}

func (e *Evaluator) structureFilterElements(elements []Element, expected string) ([]Value, bool) {
	filtered := make([]Value, 0, len(elements))
	for _, element := range elements {
		if element.IsMembership {
			continue
		}
		matches, known := e.isMetaclass(element, expected)
		if !known {
			return nil, false
		}
		if matches {
			filtered = append(filtered, elementValue(element))
		}
	}
	return filtered, true
}

func structureSequence(elements []Element) Value {
	values := make([]Value, 0, len(elements))
	for _, element := range elements {
		if element.IsMembership {
			values = append(values, membershipValue(element.Membership))
		} else {
			values = append(values, elementValue(element))
		}
	}
	return sequence(values...)
}

func structureMembershipSequence(elements []Element) (Value, bool) {
	values := make([]Value, 0, len(elements))
	for _, element := range elements {
		if !element.IsMembership {
			return Value{}, false
		}
		values = append(values, membershipValue(element.Membership))
	}
	return sequence(values...), true
}

func structureSingle(elements []Element) Value {
	switch len(elements) {
	case 0:
		return nullValue()
	case 1:
		if elements[0].IsMembership {
			return membershipValue(elements[0].Membership)
		}
		return elementValue(elements[0])
	default:
		return Value{}
	}
}

func structureRelationshipValues(elements []Value, class, property string) Value {
	switch property {
	case "source", "target", "relatedElement", "ownedRelatedElement", "client", "supplier", "annotation":
		return sequence(elements...)
	case "annotatedElement":
		if class == "AnnotatingElement" {
			return sequence(elements...)
		}
	}
	if len(elements) == 0 {
		return nullValue()
	}
	if len(elements) != 1 {
		return Value{}
	}
	return elements[0]
}

func uniqueValueSequence(values []Value) Value {
	seen := make(map[ElementKey]bool)
	unique := make([]Value, 0, len(values))
	for _, value := range values {
		if value.Kind == ElementValue {
			key := value.Element.Key()
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		unique = append(unique, value)
	}
	return sequence(unique...)
}

func slicesClone(values []Value) []Value {
	return append([]Value(nil), values...)
}
