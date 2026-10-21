package metamodel

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

type owningAnnotatingRelationshipStructure interface {
	OwningAnnotatingRelationship(Element) (Element, bool, bool)
}

type unresolvedRelatedElementStructure interface {
	HasUnresolvedRelatedElement(Element, string) bool
}

type statedRelatedElementStructure interface {
	HasStatedRelatedElement(Element, string) bool
}

type statedRelatedElementsStructure interface {
	StatedRelatedElements(Element, string) ([]Element, bool)
}

type derivedNameStructure interface {
	DerivedName(Element) (string, bool)
}

func (e *Evaluator) hasUnresolvedRelatedElement(element Element, property string) bool {
	structure, ok := e.options.Structure.(unresolvedRelatedElementStructure)
	return ok && structure.HasUnresolvedRelatedElement(element, property)
}

func (e *Evaluator) statedRelatedElements(element Element, property string) ([]Element, bool) {
	if structure, ok := e.options.Structure.(statedRelatedElementsStructure); ok {
		return structure.StatedRelatedElements(element, property)
	}
	return e.options.Structure.RelatedElements(element, property)
}

func (e *Evaluator) hasStatedRelatedElement(element Element, property string) bool {
	structure, ok := e.options.Structure.(statedRelatedElementStructure)
	return ok && structure.HasStatedRelatedElement(element, property)
}

func (e *Evaluator) structureIdentityLabel(element Element) string {
	structure, ok := e.options.Structure.(ElementIdentityStructure)
	if !ok {
		return ""
	}
	identity, ok := structure.ElementIdentity(element)
	if !ok {
		return ""
	}
	return identity
}

func (e *Evaluator) structureProperty(element Element, class, name string) (Value, bool, bool) {
	structure := e.options.Structure
	if structure == nil {
		return Value{}, false, false
	}
	if class == "Type" && name == "input" && e.model != nil {
		typeSymbol := e.structureElementSymbol(element)
		if typeSymbol != nil {
			behavior, known := e.isMetaclass(element, "Behavior")
			if known && behavior {
				if !e.completeMembers(typeSymbol) {
					e.recordMemberCompletenessFailure(element, class, name, typeSymbol)
					return Value{}, true, false
				}
				return symbolSequence(e.model.InputParametersOf(typeSymbol)), true, true
			}
		}
	}
	if class == "Type" && oneOf(name, "feature", "input") && element.Symbol != nil &&
		e.resolver != nil && e.resolver.Index() != nil {
		if _, library := identity.LibraryCatalog(e.resolver.Index()).ElementForSymbol(element.Symbol); library {
			if e.model == nil || !e.completeMembers(element.Symbol) {
				return Value{}, true, false
			}
			features, ok := e.model.ReflectiveElements(element.Symbol, "feature")
			if !ok {
				return Value{}, true, false
			}
			return symbolSequence(features), true, true
		}
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
	if name == "individualDefinition" {
		return e.structureIndividualDefinition(element)
	}
	if expected, ok := typedDefinitionMetaclasses[name]; ok &&
		!(class == "ConjugatedPortTyping" && name == "portDefinition") {
		return e.structureTypedDefinition(element, expected, typedDefinitionMany[name], class, name)
	}
	if class == "Usage" && name == "definition" {
		return e.structureTypedDefinition(element, "Classifier", true, class, name)
	}
	if class == "Import" && name == "importedElement" {
		value, ok := e.importProperty(element, name)
		return value, true, ok
	}
	if structure.Specializes(class, "InvocationExpression") && name == "operand" {
		return e.structureInvocationOperand(element)
	}
	if class == "Type" {
		switch name {
		case "feature":
			return e.structureEffectiveFeatures(element)
		case "input":
			return e.structureDirectedFeatures(element, "input")
		}
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
			return e.structureOwnedElementFilter(element, "Documentation")
		case "textualRepresentation":
			return e.structureOwnedElementFilter(element, "TextualRepresentation")
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
			members, handled, ok := e.structureOwnedMembers(element)
			if !handled || !ok {
				return Value{}, true, false
			}
			var multiplicities []Value
			for _, member := range members.Values {
				if member.Kind != ElementValue {
					return Value{}, true, false
				}
				isMultiplicity, known := e.isMetaclass(member.Element, "Multiplicity")
				if !known {
					return Value{}, true, false
				}
				if isMultiplicity {
					multiplicities = append(multiplicities, member)
				}
			}
			return structurePropertyCardinality(multiplicities, true)
		case "ownedEndFeature":
			return e.structureOwnedEndFeatures(element)
		case "endFeature":
			return e.structureEffectiveEndFeatures(element, "Type", "endFeature")
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
		case "isEnd":
			return e.structureFeatureIsEnd(element)
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
			isMultiplicity, known := e.isMetaclass(element, "Multiplicity")
			if !known {
				return Value{}, true, false
			}
			isMultiplicityRange, known := e.isMetaclass(element, "MultiplicityRange")
			if !known {
				return Value{}, true, false
			}
			if isMultiplicity || isMultiplicityRange {
				namespace, handled, ok := e.structureOwningNamespace(element)
				if !handled || !ok {
					return Value{}, true, false
				}
				if namespace.Kind != ElementValue {
					return sequence(), true, true
				}
				isFeature, known := e.isMetaclass(namespace.Element, "Feature")
				if !known {
					return Value{}, true, false
				}
				if !isFeature {
					return sequence(), true, true
				}
				value, ok := e.Property(namespace.Element, "Feature", "featuringType")
				return value, true, ok
			}
			return e.structureFeaturingTypes(
				element, make(map[ElementKey]bool), "Feature", "featuringType",
			)
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
		case "isComposite":
			if controlNode, known := e.isMetaclass(element, "ControlNode"); known && controlNode {
				return booleanValue(true), true, true
			}
			isUsage, known := e.isMetaclass(element, "Usage")
			if !known {
				return Value{}, true, false
			}
			if isUsage {
				return e.semanticBooleanAttribute(element, name)
			}
			return e.structureAttribute(element, name)
		case "isConstant":
			if stated, supported := structure.(StatedAttributeStructure); supported {
				if value, ok := stated.StatedAttribute(element, name); ok && value.Kind == BooleanValue {
					return value, true, true
				}
			} else if value, ok := structure.Attribute(element, name); ok && value.Kind == BooleanValue {
				return value, true, true
			}
			if element.Symbol == nil || e.model == nil {
				return Value{}, true, false
			}
			if e.model.FeatureIsConstant(element.Symbol) {
				return booleanValue(true), true, true
			}
			isUsage, known := e.isMetaclass(element, "Usage")
			if !known {
				return Value{}, true, false
			}
			if !isUsage {
				return booleanValue(false), true, true
			}
			isEnd, ok := e.model.ReflectiveFeatureValue(element.Symbol, "isEnd")
			if !ok || isEnd.Kind != symbols.FilterValueBool {
				return Value{}, true, false
			}
			if !isEnd.Bool {
				return booleanValue(false), true, true
			}
			mayTimeVary, ok := e.Property(element, "Usage", "mayTimeVary")
			if !ok || mayTimeVary.Kind != BooleanValue {
				return Value{}, true, false
			}
			return booleanValue(mayTimeVary.Boolean), true, true
		case "isVariable":
			if element.Symbol != nil && e.model != nil && e.model.IsKerMLFeature(element.Symbol) {
				return booleanValue(e.model.FeatureIsVariable(element.Symbol)), true, true
			}
			isUsage, known := e.isMetaclass(element, "Usage")
			if !known {
				if element.Symbol == nil || e.model == nil {
					return Value{}, true, false
				}
				return booleanValue(e.model.FeatureIsVariable(element.Symbol)), true, true
			}
			if isUsage {
				if element.Symbol == nil {
					return Value{}, true, false
				}
				value, ok := e.Property(element, "Usage", "mayTimeVary")
				if !ok {
					e.recordUnsupportedReason(
						element, "Feature", "isVariable",
						Reason{Category: ReasonDependsOnUnsupported, Dependency: "Usage::mayTimeVary"},
					)
				}
				return value, true, ok
			}
			if value, ok := structure.Attribute(element, name); ok {
				if value.Kind == BooleanValue {
					return value, true, true
				}
				return Value{}, true, false
			}
			if element.Symbol == nil {
				return booleanValue(false), true, true
			}
			if e.model == nil {
				return Value{}, true, false
			}
			return booleanValue(e.model.FeatureIsVariable(element.Symbol)), true, true
		}
		if strings.HasPrefix(name, "owned") {
			if rangeClass, ok := structureRange(class, name); ok {
				return e.structureOwnedFilter(element, rangeClass, false)
			}
		}
	case "AnnotatingElement":
		switch name {
		case "annotation":
			return e.structureAnnotation(element)
		case "annotatedElement":
			return e.structureAnnotatedElements(element)
		case "ownedAnnotatingRelationship":
			return e.structureOwnedAnnotatingRelationships(element)
		case "owningAnnotatingRelationship":
			return e.structureOwningAnnotatingRelationship(element)
		}
		if strings.HasPrefix(name, "owned") {
			if rangeClass, ok := structureRange(class, name); ok && rangeClass != "Feature" {
				return e.structureOwnedFilter(element, rangeClass, false)
			}
		}
	case "Annotation":
		switch name {
		case "annotatingElement":
			return e.structureAnnotationAnnotatingElement(element)
		case "ownedAnnotatingElement":
			return e.structureAnnotationOwnedAnnotatingElement(element)
		case "owningAnnotatingElement":
			return e.structureAnnotationOwningAnnotatingElement(element)
		}
	case "Usage":
		switch name {
		case "owningUsage":
			return e.structureOwningTypeOf(element, "Usage")
		case "owningDefinition":
			return e.structureOwningTypeOf(element, "Definition")
		case "usage":
			value, handled, ok := e.structureFilteredEffectiveFeatures(element, "Usage")
			if !handled || !ok {
				e.recordUnsupportedReason(
					element, "Usage", name,
					Reason{
						Category: ReasonUnrepresentedEndpoint,
						Detail:   "Usage::usage effective FeatureMembership memberElement endpoints are unavailable",
					},
				)
			}
			return value, handled, ok
		case "directedUsage":
			return e.structureDirectedUsages(element)
		case "variant", "variantMembership":
			return e.structureVariantProperty(element, class, name)
		case "isReference", "mayTimeVary":
			return e.semanticBooleanAttribute(element, name)
		}
		if strings.HasPrefix(name, "nested") {
			return e.structureFilteredOwnedFeatures(element, name, "nested")
		}
	case "AcceptActionUsage":
		switch name {
		case "payloadParameter":
			return e.structureFirstParameter(element)
		case "payloadArgument":
			return e.structureActionArgument(element, "payloadArgument", 1)
		case "receiverArgument":
			return e.structureActionArgument(element, "receiverArgument", 2)
		}
	case "SendActionUsage":
		switch name {
		case "payloadArgument":
			return e.structureActionArgument(element, "payloadArgument", 1)
		case "senderArgument":
			return e.structureActionArgument(element, "senderArgument", 2)
		case "receiverArgument":
			return e.structureActionArgument(element, "receiverArgument", 3)
		}
	case "PerformActionUsage":
		if name == "performedAction" {
			return e.structurePerformedAction(element)
		}
	case "ExhibitStateUsage":
		if name == "exhibitedState" {
			return e.structurePerformedAction(element)
		}
	case "Definition":
		switch name {
		case "usage":
			return e.structureFilteredEffectiveFeatures(element, "Usage")
		case "directedUsage":
			return e.structureDirectedUsages(element)
		case "variant", "variantMembership":
			return e.structureVariantProperty(element, class, name)
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
	case "FeatureReferenceExpression":
		if name == "referent" {
			return e.structureFeatureReferenceReferent(element)
		}
	case "FeatureChainExpression":
		switch name {
		case "targetFeature":
			return e.structureFeatureReferenceReferent(element)
		case "operator":
			return stringValue("."), true, true
		}
	case "Expression":
		switch name {
		case "function":
			expressionNode := expressionNodeOf(element)
			if operator, ok := expressionNode.(*ast.OperatorExpr); ok {
				if e.model == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonMissingSemanticModel,
							Detail:   "Expression::function operator resolution requires a semantic model",
						},
					)
					return Value{}, true, false
				}
				function := e.model.OperatorFunction(operator.Operator)
				if function == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonUnresolvedOperatorFunction,
							Detail:   "Expression::function operator has no resolved library function",
						},
					)
					return Value{}, true, false
				}
				return elementValue(ElementOf(function)), true, true
			}
			if index, ok := expressionNode.(*ast.IndexExpr); ok && index.Bracket {
				if e.model == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonMissingSemanticModel,
							Detail:   "Expression::function operator resolution requires a semantic model",
						},
					)
					return Value{}, true, false
				}
				function := e.model.OperatorFunction(ast.OpIndex)
				if function == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonUnresolvedOperatorFunction,
							Detail:   "Expression::function operator '[' has no resolved library function",
						},
					)
					return Value{}, true, false
				}
				return elementValue(ElementOf(function)), true, true
			}
			if _, ok := expressionNode.(*ast.SequenceExpr); ok {
				if e.model == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonMissingSemanticModel,
							Detail:   "Expression::function operator resolution requires a semantic model",
						},
					)
					return Value{}, true, false
				}
				function := e.model.OperatorFunctionNamed(",")
				if function == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonUnresolvedOperatorFunction,
							Detail:   "Expression::function operator ',' has no resolved library function",
						},
					)
					return Value{}, true, false
				}
				return elementValue(ElementOf(function)), true, true
			}
			if _, ok := expressionNode.(*ast.FeatureChainExpr); ok {
				if e.model == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonMissingSemanticModel,
							Detail:   "Expression::function operator resolution requires a semantic model",
						},
					)
					return Value{}, true, false
				}
				function := e.model.OperatorFunctionNamed(".")
				if function == nil {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category: ReasonUnresolvedOperatorFunction,
							Detail:   "Expression::function operator '.' has no resolved library function",
						},
					)
					return Value{}, true, false
				}
				return elementValue(ElementOf(function)), true, true
			}
			if constructor, known := e.isMetaclass(element, "ConstructorExpression"); !known {
				e.recordUnsupportedReason(
					element, "Expression", name,
					Reason{
						Category: ReasonUnrepresentedElement,
						Detail:   "Expression::function expression metaclass is unavailable",
					},
				)
				return Value{}, true, false
			} else if constructor {
				e.recordUnsupportedReason(
					element, "Expression", name,
					Reason{
						Category: ReasonNotApplicable,
						Detail:   "Expression::function is not defined for a ConstructorExpression",
					},
				)
				return Value{}, true, false
			}
			function, handled, ok := e.structureTypedFeatureTypes(element, "Function", true)
			if !handled || !ok || function.Kind != NullValue {
				return function, handled, ok
			}
			instantiated, ok := e.instantiationExpressionProperty(element, "instantiatedType")
			if !ok {
				if _, invocation := expressionNode.(*ast.InvocationExpr); invocation {
					e.recordUnsupportedReason(
						element, "Expression", name,
						Reason{
							Category:   ReasonDependsOnUnsupported,
							Dependency: "InstantiationExpression::instantiatedType",
						},
					)
					return Value{}, true, false
				}
				return function, true, true
			}
			if instantiated.Kind != ElementValue {
				return function, true, true
			}
			isFunction, known := e.isMetaclass(instantiated.Element, "Function")
			if !known {
				e.recordUnsupportedReason(
					element, "Expression", name,
					Reason{
						Category: ReasonUnrepresentedElement,
						Detail:   "Expression::function instantiatedType has no represented metaclass",
					},
				)
				return Value{}, true, false
			}
			if isFunction {
				return instantiated, true, true
			}
			return function, true, true
		case "result":
			return e.structureResultFeatures(element, "Expression", "result")
		case "isModelLevelEvaluable":
			return e.structureAttribute(element, name)
		}
	case "InstantiationExpression":
		switch name {
		case "argument":
			value, handled, ok := e.structureInvocationArguments(element)
			if !handled || !ok {
				e.recordUnsupportedReason(
					element, "InstantiationExpression", name,
					Reason{
						Category: ReasonIncompleteStructure,
						Detail:   "InstantiationExpression::argument actual arguments cannot be bound to formal input Parameters",
					},
				)
			}
			return value, handled, ok
		case "instantiatedType":
			value, ok := e.instantiationExpressionProperty(element, name)
			return value, true, ok
		}
	case "Function":
		switch name {
		case "expression":
			return e.structureFilteredEffectiveFeatures(element, "Expression")
		case "result":
			return e.structureResultFeatures(element, "Function", "result")
		case "isModelLevelEvaluable":
			return e.structureAttribute(element, name)
		}
	case "Connector":
		switch name {
		case "connectorEnd":
			return e.structureEffectiveEndFeatures(element, "Connector", "connectorEnd")
		case "relatedFeature":
			return e.structureConnectorRelatedFeatures(element)
		case "sourceFeature":
			return e.structureConnectorEndpoint(element, "source")
		case "targetFeature":
			return e.structureConnectorEndpoint(element, "target")
		case "association":
			return e.structureTypedFeatureTypes(element, "Association", false)
		case "defaultFeaturingType":
			return e.structureFeaturingTypes(
				element, make(map[ElementKey]bool), "Connector", "defaultFeaturingType",
			)
		default:
			return e.structureRelationshipProperty(element, class, name)
		}
	case "ConjugatedPortDefinition":
		switch name {
		case "ownedPortConjugator":
			conjugators, handled, ok := e.structureOwnedFilter(element, "PortConjugation", false)
			if !handled || !ok {
				return Value{}, true, false
			}
			return structurePropertyCardinality(conjugators.Values, true)
		case "originalPortDefinition":
			namespace, handled, ok := e.structureOwningNamespace(element)
			if !handled || !ok || namespace.Kind != ElementValue {
				return namespace, true, ok
			}
			isPortDefinition, known := e.isMetaclass(namespace.Element, "PortDefinition")
			if !known {
				return Value{}, true, false
			}
			if !isPortDefinition {
				return nullValue(), true, true
			}
			return namespace, true, true
		}
	case "PortDefinition":
		if name == "conjugatedPortDefinition" {
			memberships, handled, ok := e.structureOwnedFilter(element, "Membership", true)
			if !handled || !ok {
				return Value{}, true, false
			}
			var members []Element
			for _, membership := range memberships.Values {
				if membership.Kind != MembershipValue {
					return Value{}, true, false
				}
				found, ok := e.structureMembershipMembers(MembershipElement(membership.Membership))
				if !ok {
					return Value{}, true, false
				}
				members = append(members, found...)
			}
			filtered, ok := e.structureFilterElements(members, "ConjugatedPortDefinition")
			if !ok {
				return Value{}, true, false
			}
			return structurePropertyCardinality(filtered, true)
		}
	case "ConjugatedPortTyping":
		if name == "portDefinition" {
			targets, ok := e.relationshipEndpoints(element, "conjugatedPortDefinition")
			if !ok {
				return Value{}, true, false
			}
			filtered, ok := e.structureFilterValues(targets, "ConjugatedPortDefinition")
			if !ok {
				return Value{}, true, false
			}
			conjugated, handled, ok := structurePropertyCardinality(filtered, true)
			if !handled || !ok || conjugated.Kind == NullValue {
				return conjugated, true, ok
			}
			if conjugated.Kind != ElementValue {
				return Value{}, true, false
			}
			value, ok := e.Property(conjugated.Element, "ConjugatedPortDefinition", "originalPortDefinition")
			return value, true, ok
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
	case "name":
		return e.structureName(element, "name")
	case "shortName":
		return e.structureName(element, "shortName")
	case "ownedMemberName":
		value, handled, ok := e.structureMemberName(element, "", "name")
		if handled && !ok {
			e.recordUnsupportedReason(
				element, "OwningMembership", "ownedMemberName",
				Reason{Category: ReasonDependsOnUnsupported, Dependency: "Element::name"},
			)
		}
		return value, handled, ok
	case "ownedMemberShortName":
		value, handled, ok := e.structureMemberName(element, "", "shortName")
		if handled && !ok {
			e.recordUnsupportedReason(
				element, "OwningMembership", "ownedMemberShortName",
				Reason{Category: ReasonDependsOnUnsupported, Dependency: "Element::shortName"},
			)
		}
		return value, handled, ok
	case "memberName", "memberShortName":
		isOwningMembership, known := e.isMetaclass(element, "OwningMembership")
		if !known {
			return Value{}, true, false
		}
		if isOwningMembership {
			memberProperty := "name"
			redefiningProperty := "ownedMemberName"
			if name == "memberShortName" {
				memberProperty = "shortName"
				redefiningProperty = "ownedMemberShortName"
			}
			value, handled, ok := e.structureMemberName(element, "", memberProperty)
			if handled && !ok {
				e.recordUnsupportedReason(
					element, "Membership", name,
					Reason{
						Category:   ReasonDependsOnUnsupported,
						Dependency: "OwningMembership::" + redefiningProperty,
					},
				)
			}
			return value, handled, ok
		}
		return e.structureAttribute(element, name)
	case "featureWithValue":
		values, ok := e.relationshipEndpoints(element, "membershipOwningNamespace")
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(values, "Feature")
		if !ok {
			return Value{}, true, false
		}
		return structurePropertyCardinality(filtered, true)
	case "value":
		values, ok := e.relationshipEndpoints(element, "memberElement")
		if !ok {
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(values, "Expression")
		if !ok {
			return Value{}, true, false
		}
		return structurePropertyCardinality(filtered, true)
	case "memberElementId", "ownedMemberElementId":
		members, ok := e.structureMembershipMembers(element)
		if !ok || len(members) != 1 {
			e.recordUnsupportedReason(
				element, class, name,
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   class + "::" + name + " requires exactly one represented memberElement",
				},
			)
			return Value{}, true, false
		}
		identity, ok := e.options.Structure.(ElementIdentityStructure)
		if !ok {
			e.recordUnsupportedReason(
				element, class, name,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + name + " Structure does not provide memberElement identity",
				},
			)
			return Value{}, true, false
		}
		id, ok := identity.ElementIdentity(members[0])
		if !ok || id == "" {
			e.recordUnsupportedReason(
				element, class, name,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + name + " memberElement has no stable identity",
				},
			)
			return Value{}, true, false
		}
		return stringValue(id), true, true
	case "memberElement", "ownedMemberElement", "ownedMemberFeature",
		"ownedMemberParameter", "ownedActorParameter", "ownedSubjectParameter",
		"ownedStakeholderParameter", "ownedResultExpression",
		"ownedObjectiveRequirement", "ownedRequirement", "verifiedRequirement",
		"ownedVariantUsage", "ownedConstraint", "referencedConstraint",
		"ownedConcern", "referencedConcern", "ownedRendering", "referencedRendering",
		"action", "transitionFeature", "condition":
		expected := map[string]string{
			"ownedMemberElement":        "",
			"ownedMemberFeature":        "Feature",
			"ownedMemberParameter":      "Feature",
			"ownedActorParameter":       "PartUsage",
			"ownedStakeholderParameter": "PartUsage",
			"ownedSubjectParameter":     "Usage",
			"ownedResultExpression":     "Expression",
			"ownedObjectiveRequirement": "RequirementUsage",
			"ownedRequirement":          "RequirementUsage",
			"verifiedRequirement":       "RequirementUsage",
			"ownedVariantUsage":         "Usage",
			"ownedConstraint":           "ConstraintUsage",
			"referencedConstraint":      "ConstraintUsage",
			"ownedConcern":              "ConcernUsage",
			"referencedConcern":         "ConcernUsage",
			"ownedRendering":            "RenderingUsage",
			"referencedRendering":       "RenderingUsage",
			"action":                    "ActionUsage",
			"transitionFeature":         "Step",
			"condition":                 "Expression",
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
		return structurePropertyCardinality(filtered, true)
	case "owningType":
		return e.structureRelationshipOwner(element, "Type")
	}
	return e.structureRelationshipProperty(element, class, name)
}

func (e *Evaluator) structureFeatureChaining(element Element) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, "Feature", "chainingFeature", reason)
		return Value{}, true, false
	}
	relationships, handled, ok := e.structureOwnedFilter(element, "FeatureChaining", false)
	if !handled || !ok {
		return unsupported(Reason{
			Category: ReasonMissingStructure,
			Detail:   "Feature::chainingFeature owned FeatureChaining relationships are unavailable",
		})
	}
	var values []Value
	for _, relationship := range relationships.Values {
		if relationship.Kind != ElementValue {
			return unsupported(Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Feature::chainingFeature FeatureChaining relationship is not represented as an Element",
			})
		}
		endpoints, ok := e.relationshipEndpoints(relationship.Element, "chainingFeature")
		if !ok {
			if e.hasUnresolvedRelatedElement(relationship.Element, "chainingFeature") {
				return unsupported(Reason{
					Category: ReasonUnresolvedReference,
					Detail:   "Feature::chainingFeature target reference is unresolved",
				})
			}
			return unsupported(Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Feature::chainingFeature target is unavailable in Structure",
			})
		}
		if len(endpoints) == 0 {
			return unsupported(Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Feature::chainingFeature has no represented target",
			})
		}
		filtered, ok := e.structureFilterValues(endpoints, "Feature")
		if !ok {
			return unsupported(Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Feature::chainingFeature target metaclass is unavailable",
			})
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
		value, handled, ok := e.structureEffectiveMembershipMembers(
			element, "SubjectMembership", "ownedSubjectParameter", "Usage", true,
		)
		if !handled || !ok {
			e.recordUnsupportedReason(
				element, "RequirementUsage", "subjectParameter",
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "RequirementUsage::subjectParameter SubjectMembership ownedSubjectParameter endpoint is unavailable",
				},
			)
		}
		return value, handled, ok
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
	documentation, handled, ok := e.structureOwnedElementFilter(element, "Documentation")
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

func (e *Evaluator) structureInvocationOperand(element Element) (Value, bool, bool) {
	invocation, ok := element.Node.(*ast.InvocationExpr)
	if !ok {
		operands, known := e.options.Structure.RelatedElements(element, "operand")
		if !known {
			e.recordUnsupportedReason(
				element, "InvocationExpression", "operand",
				Reason{
					Category: ReasonMissingStructure,
					Detail:   "InvocationExpression::operand receiver is not represented in Structure",
				},
			)
			return Value{}, true, false
		}
		value, _, ok := e.structureFilterRelated(operands, "Expression", false)
		if !ok {
			e.recordUnsupportedReason(
				element, "InvocationExpression", "operand",
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "InvocationExpression::operand does not resolve to Expression elements",
				},
			)
		}
		return value, true, ok
	}
	if invocation.Operand == nil {
		return sequence(), true, true
	}
	operand := Element{Node: invocation.Operand, Container: element.Container}
	isExpression, known := e.isMetaclass(operand, "Expression")
	if !known || !isExpression {
		return Value{}, true, false
	}
	return sequence(elementValue(operand)), true, true
}

func (e *Evaluator) structureTransitionEndpoint(element Element, endpoint string) (Value, bool, bool) {
	if e.hasUnresolvedRelatedElement(element, endpoint) {
		e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "compact transition endpoint is unresolved",
		})
		return Value{}, true, false
	}
	if e.hasStatedRelatedElement(element, endpoint) {
		related, ok := e.statedRelatedElements(element, endpoint)
		if !ok || len(related) == 0 {
			e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "stated compact transition endpoint is unresolved",
			})
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(related, "ActionUsage")
		if !ok {
			e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "stated compact transition endpoint has an unresolved range",
			})
			return Value{}, true, false
		}
		return structurePropertyCardinality(filtered, true)
	}
	node := structureElementNode(element)
	owner, known := e.structureSemanticOwnerSymbol(element)
	if !known {
		e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "transition owner is unresolved",
		})
		return Value{}, true, false
	}
	for _, succession := range e.model.ActionSuccessions(owner) {
		if succession.Decl != node {
			continue
		}
		end := succession.Source
		if endpoint == "target" {
			end = succession.Target
		}
		target := end.Symbol
		if target == nil && end.Node != nil {
			if _, final := end.Node.(*ast.FinalNode); final {
				target, known = e.structureFinalNodeSymbol()
				if !known {
					e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
						Category: ReasonUnresolvedReference,
						Detail:   "final transition endpoint is unresolved",
					})
					return Value{}, true, false
				}
			}
			for _, member := range e.model.MembersOf(owner) {
				if target == nil && member != nil && member.Decl == end.Node {
					target = member
					break
				}
			}
		}
		if target == nil {
			transition, isTransition := node.(*ast.TransitionMember)
			if endpoint == "source" && isTransition && transition.Source == nil {
				return elementValue(ElementOf(owner)), true, true
			}
			detail := "transition endpoint is unresolved"
			if e.hasStatedRelatedElement(element, endpoint) {
				detail += "; compact endpoint is graph-stated"
			}
			e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
				Category: ReasonUnresolvedReference,
				Detail:   detail,
			})
			return Value{}, true, false
		}
		return elementValue(ElementOf(target)), true, true
	}
	e.recordUnsupportedReason(element, "TransitionUsage", endpoint, Reason{
		Category: ReasonIncompleteStructure,
		Detail:   "transition has no semantic action succession",
	})
	return Value{}, true, false
}

func (e *Evaluator) structureMemberCompletenessReason(sym *symbols.Symbol) Reason {
	if !e.completeRelationships(sym) {
		return Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "specialization or reference subsetting target is unresolved",
		}
	}
	if e.model.SupertypesProvisional(sym) {
		return Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "provisional supertypes leave inherited membership sources unknown",
		}
	}
	if e.model.HasSpecializationCycle(sym) {
		return Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "specialization cycle prevents a complete inherited membership set",
		}
	}
	supers := append([]*symbols.Symbol{sym}, e.model.AllSupertypes(sym)...)
	for _, super := range supers {
		if e.model.SupertypesProvisional(super) {
			return Reason{
				Category: ReasonIncompleteStructure,
				Detail:   "supertype has provisional supertypes",
			}
		}
		if e.model.HasSpecializationCycle(super) {
			return Reason{
				Category: ReasonIncompleteStructure,
				Detail:   "supertype has a specialization cycle",
			}
		}
		for _, base := range e.model.KindBaseFQNs(
			super, source.KindOf(symbols.DocNameOf(super.Scope)) == source.KindKerML,
		) {
			if e.resolver == nil || e.resolver.Index() == nil ||
				len(e.resolver.Index().LookupQualified(base)) == 0 {
				return Reason{
					Category: ReasonUnresolvedReference,
					Detail:   "cannot resolve supertype base " + base,
				}
			}
		}
	}
	e.model.MemberSources(sym)
	if !e.model.MemberSourcesStable(sym) {
		return Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "inherited membership sources are unstable",
		}
	}
	return Reason{
		Category: ReasonIncompleteStructure,
		Detail:   "inherited membership source set is incomplete",
	}
}

func (e *Evaluator) recordMemberCompletenessFailure(
	element Element, class, property string, sym *symbols.Symbol,
) {
	e.recordUnsupportedReason(
		element, class, property,
		e.structureMemberCompletenessReason(sym),
	)
}

func (e *Evaluator) structureFinalNodeSymbol() (*symbols.Symbol, bool) {
	if e.resolver == nil || e.resolver.Index() == nil {
		return nil, false
	}
	index := e.resolver.Index()
	catalog := identity.LibraryCatalog(index)
	var target *symbols.Symbol
	for _, candidate := range index.LookupQualified("Actions::Action::done") {
		if _, library := catalog.ElementForSymbol(candidate); !library {
			continue
		}
		if target != nil {
			return nil, false
		}
		target = candidate
	}
	return target, target != nil
}

func (e *Evaluator) structureActionSuccessionFeatures(element Element) (Value, bool, bool) {
	node := structureElementNode(element)
	switch succession := node.(type) {
	case *ast.Usage:
		if succession.Kind != ast.UsageSuccession && !succession.IsSuccessionFlow() ||
			succession.Kind == ast.UsageSuccession && len(succession.ConnectorEnds) != 2 {
			return Value{}, false, true
		}
	case *ast.InitialNode:
		if succession.Successor == nil {
			return Value{}, false, true
		}
	case *ast.SuccessionEdge, *ast.ControlFlowEdge:
	case *ast.TransitionMember:
		if succession.Target == nil {
			return Value{}, false, true
		}
	default:
		return Value{}, false, true
	}
	owners := make([]*symbols.Symbol, 0, 3)
	owner, ownerKnown := e.structureSemanticOwnerSymbol(element)
	if ownerKnown && owner != nil {
		owners = append(owners, owner)
	}
	if element.Container != nil {
		owners = append(owners, element.Container)
		if element.Container.Decl == node {
			if parent := element.Container.Owner(); parent != nil {
				owners = append(owners, parent)
			}
		}
	}
	if owner, found := e.structureActionSuccessionOwner(node); found && owner != nil {
		owners = append(owners, owner)
	}
	if len(owners) == 0 {
		return Value{}, false, false
	}
	seenOwners := make(map[*symbols.Symbol]bool, len(owners))
	for _, owner := range owners {
		if owner == nil || seenOwners[owner] {
			continue
		}
		seenOwners[owner] = true
		for _, succession := range e.model.ActionSuccessions(owner) {
			if succession.Decl != node {
				continue
			}
			values := make([]Value, 0, 2)
			for _, end := range []semantics.ActionSuccessionEnd{succession.Source, succession.Target} {
				target := end.Symbol
				if target == nil && end.Node != nil {
					if _, final := end.Node.(*ast.FinalNode); final {
						finalTarget, ok := e.structureFinalNodeSymbol()
						if !ok {
							return Value{}, true, false
						}
						target = finalTarget
					}
					for _, member := range e.model.MembersOf(owner) {
						if target == nil && member != nil && member.Decl == end.Node {
							target = member
							break
						}
					}
				}
				if target == nil {
					switch end.Node.(type) {
					case *ast.InitialNode, *ast.EntryMember:
						values = append(values, nullValue())
						continue
					}
					if end.Node == nil && end.Symbol == nil && end.Span.Len == 0 {
						switch succession := node.(type) {
						case *ast.InitialNode:
							if len(values) == 0 {
								target = owner
							}
						case *ast.TransitionMember:
							if len(values) == 0 && succession.Source == nil {
								target = owner
							}
						}
						if target == nil {
							values = append(values, nullValue())
							continue
						}
					} else {
						return Value{}, true, false
					}
				}
				values = append(values, elementValue(ElementOf(target)))
			}
			return sequence(values...), true, true
		}
	}
	if !ownerKnown {
		return Value{}, false, false
	}
	return Value{}, false, true
}

func (e *Evaluator) structureActionSuccessionOwner(node ast.Node) (*symbols.Symbol, bool) {
	if node == nil || e.model == nil || e.resolver == nil || e.resolver.Index() == nil {
		return nil, false
	}
	switch succession := node.(type) {
	case *ast.InitialNode, *ast.SuccessionEdge, *ast.ControlFlowEdge, *ast.TransitionMember:
	case *ast.Usage:
		if succession.Kind != ast.UsageSuccession && !succession.IsSuccessionFlow() {
			return nil, false
		}
	default:
		return nil, false
	}
	index := e.resolver.Index()
	visited := make(map[*symbols.Scope]bool)
	var find func(*symbols.Scope) (*symbols.Symbol, bool)
	find = func(scope *symbols.Scope) (*symbols.Symbol, bool) {
		if scope == nil || visited[scope] {
			return nil, false
		}
		visited[scope] = true
		for _, candidate := range scope.AllMembers() {
			if candidate == nil {
				continue
			}
			if semantics.ActionDeclaration(candidate.Decl) {
				for _, succession := range e.model.ActionSuccessions(candidate) {
					if succession.Decl == node && succession.Owner != nil {
						return succession.Owner, true
					}
				}
			}
			if owner, found := find(candidate.Scope); found {
				return owner, true
			}
		}
		for _, child := range scope.Children() {
			if owner, found := find(child); found {
				return owner, true
			}
		}
		return nil, false
	}
	for _, document := range index.WorkspaceDocuments() {
		if owner, found := find(index.DocumentRoot(document)); found {
			return owner, true
		}
	}
	return nil, false
}

func (e *Evaluator) structureSemanticOwnerSymbol(element Element) (*symbols.Symbol, bool) {
	owner, present, known := e.structureOwnerElement(element)
	if !known || !present || e.model == nil {
		return e.structureActionSuccessionOwner(structureElementNode(element))
	}
	if owner.Symbol != nil {
		return owner.Symbol, true
	}
	if owner.Node == nil {
		return e.structureActionSuccessionOwner(structureElementNode(element))
	}
	var scope *symbols.Scope
	if owner.Container != nil {
		scope = owner.Container.Scope
	}
	symbol, found := e.model.SymbolDeclaring(scope, owner.Node)
	if found && symbol != nil {
		return symbol, true
	}
	return e.structureActionSuccessionOwner(structureElementNode(element))
}

func structureElementNode(element Element) ast.Node {
	if element.Node != nil {
		return element.Node
	}
	if element.Symbol != nil {
		return element.Symbol.Decl
	}
	return nil
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
	var value bool
	switch property {
	case "isComposite", "isReference":
		composite, handled, ok := e.structureUsageComposite(element)
		if handled {
			if !ok {
				if property == "isReference" {
					reason := Reason{
						Category: ReasonIncompleteStructure,
						Detail:   "Usage::isReference owning Type classification is unavailable",
					}
					if element.Symbol == nil || e.model == nil {
						reason = Reason{
							Category: ReasonMissingSemanticModel,
							Detail:   "Usage::isReference has no semantic Usage symbol to determine referentiality",
						}
					}
					e.recordUnsupportedReason(element, "Usage", property, reason)
				}
				return Value{}, true, false
			}
			value = composite
		} else {
			if element.Symbol == nil || e.model == nil {
				return Value{}, true, false
			}
			value = e.model.EffectiveIsComposite(element.Symbol)
		}
		if property == "isReference" {
			value = !value
		}
	case "isVariable":
		if element.Symbol == nil || e.model == nil {
			return Value{}, true, false
		}
		value = e.model.FeatureIsVariable(element.Symbol)
	case "mayTimeVary":
		owningType, handled, ok := e.structureOwningType(element)
		if !handled || !ok {
			e.recordUnsupportedReason(
				element, "Usage", "mayTimeVary",
				Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Usage::mayTimeVary has no resolvable owning Type",
				},
			)
			return Value{}, true, false
		}
		if owningType.Kind != ElementValue {
			return booleanValue(false), true, true
		}
		if element.Symbol == nil || owningType.Element.Symbol == nil || e.model == nil {
			e.recordUnsupportedReason(
				element, "Usage", "mayTimeVary",
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Usage::mayTimeVary requires semantic Usage and owning Type symbols",
				},
			)
			return Value{}, true, false
		}
		value = e.model.UsageMayTimeVaryForOwningType(element.Symbol, owningType.Element.Symbol)
	default:
		reflective, ok := e.model.ReflectiveFeatureValue(element.Symbol, property)
		if !ok || reflective.Kind != symbols.FilterValueBool {
			return Value{}, true, false
		}
		value = reflective.Bool
	}
	return booleanValue(value), true, true
}

func (e *Evaluator) structureUsageComposite(element Element) (bool, bool, bool) {
	controlNode, known := e.isMetaclass(element, "ControlNode")
	if !known {
		return false, true, false
	}
	if controlNode {
		return true, true, true
	}
	// These metaclasses have explicit reference rules in SysML.uml:
	// validateAttributeUsageIsReference (9670–9676),
	// validateEventOccurrenceUsageIsReference (12559–12565),
	// validatePerformActionUsageReference (11964–11973),
	// validateExhibitStateUsageReference (8641–8650),
	// validateIncludeUseCaseUsageReference (15601–15610), and
	// validateReferenceUsageIsReference (14735–14741).
	for _, nonComposite := range []string{
		"AttributeUsage",
		"EventOccurrenceUsage",
		"PerformActionUsage",
		"ExhibitStateUsage",
		"IncludeUseCaseUsage",
		"ReferenceUsage",
	} {
		matches, known := e.isMetaclass(element, nonComposite)
		if !known {
			return false, true, false
		}
		if matches {
			return false, true, true
		}
	}
	if element.Symbol != nil && e.model != nil {
		return e.model.EffectiveIsComposite(element.Symbol), true, true
	}
	if e.options.Structure == nil {
		return false, true, false
	}
	membership, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return false, true, false
	}
	if present {
		for _, requiredComposite := range []string{
			"ObjectiveMembership",
			"RequirementConstraintMembership",
		} {
			matches, known := e.isMetaclass(membership, requiredComposite)
			if !known {
				return false, true, false
			}
			if matches {
				return true, true, true
			}
		}
		for _, defaultInputMembership := range []string{
			"SubjectMembership",
			"ActorMembership",
			"StakeholderMembership",
		} {
			matches, known := e.isMetaclass(membership, defaultInputMembership)
			if !known {
				return false, true, false
			}
			if matches {
				return false, true, true
			}
		}
		endMembership, known := e.isMetaclass(membership, "EndFeatureMembership")
		if !known {
			return false, true, false
		}
		if endMembership {
			return false, true, true
		}
	}
	direction, known := e.options.Structure.Attribute(element, "direction")
	if !known {
		return false, true, false
	}
	if direction.Kind != NullValue {
		return false, true, true
	}
	isEnd, known := e.options.Structure.Attribute(element, "isEnd")
	if !known {
		return false, true, false
	}
	if isEnd.Kind == BooleanValue && isEnd.Boolean {
		return false, true, true
	}
	owningType, handled, known := e.structureOwningType(element)
	if !handled || !known {
		return false, true, false
	}
	if owningType.Kind == NullValue {
		if present {
			return false, true, true
		}
		return false, true, false
	}
	if owningType.Kind != ElementValue {
		return false, true, false
	}
	composite, known := e.options.Structure.Attribute(element, "isComposite")
	if !known || composite.Kind != BooleanValue {
		return false, true, false
	}
	return composite.Boolean, true, true
}

func (e *Evaluator) structureName(element Element, property string) (Value, bool, bool) {
	return e.structureNameWithSeen(element, property, make(map[ElementKey]bool))
}

func (e *Evaluator) structureNameWithSeen(element Element, property string, seen map[ElementKey]bool) (Value, bool, bool) {
	effectiveProperty := property
	key := element.Key()
	if seen[key] {
		return nullValue(), true, true
	}
	seen[key] = true
	defer delete(seen, key)
	if property == "name" || property == "shortName" {
		name, shortName, declared := e.structureDeclaredIdentifiers(element)
		if !element.IsMembership && declared {
			if property == "name" {
				if name != "" {
					return stringValue(name), true, true
				}
				return nullValue(), true, true
			}
			if shortName != "" {
				return stringValue(shortName), true, true
			}
			return nullValue(), true, true
		}
	}
	if property == "name" {
		isExhibitStateUsage, known := e.isMetaclass(element, "ExhibitStateUsage")
		if known && isExhibitStateUsage {
			if _, handled, ok := e.structureReferencedFeature(element); handled && !ok {
				return nullValue(), true, true
			}
		}
		isConjugatedDefinition, known := e.isMetaclass(element, "ConjugatedPortDefinition")
		if known && isConjugatedDefinition {
			original, ok := e.Property(element, "ConjugatedPortDefinition", "originalPortDefinition")
			if !ok {
				return Value{}, true, false
			}
			if original.Kind != ElementValue {
				return nullValue(), true, true
			}
			name, ok := e.Property(original.Element, "Element", "name")
			if !ok {
				return Value{}, true, false
			}
			if name.Kind != StringValue || name.String == "" {
				return nullValue(), true, true
			}
			return stringValue("~" + name.String), true, true
		}
		if !known {
			return Value{}, true, false
		}
	}
	if (property == "name" || property == "shortName") && e.options.Structure != nil {
		declaredProperty := "declaredName"
		otherDeclaredProperty := "declaredShortName"
		if property == "shortName" {
			declaredProperty, otherDeclaredProperty = otherDeclaredProperty, declaredProperty
		}
		if value, ok := e.options.Structure.Attribute(element, declaredProperty); ok &&
			value.Kind == StringValue && value.String != "" {
			return value, true, true
		}
		if value, ok := e.options.Structure.Attribute(element, otherDeclaredProperty); ok &&
			value.Kind == StringValue && value.String != "" &&
			!(property == "shortName" && element.Symbol != nil &&
				element.Symbol.Naming == symbols.NamedByRedefinition) {
			return nullValue(), true, true
		}
		if property == "name" {
			if structure, ok := e.options.Structure.(derivedNameStructure); ok {
				if name, present := structure.DerivedName(element); present && name != "" {
					return stringValue(name), true, true
				}
			}
		}
		if property == "name" && element.Symbol != nil && !element.IsMembership &&
			element.Symbol.Naming == symbols.NamedByDeclaration {
			if name, shortName, declared := symbolDeclaredNames(element.Symbol); declared &&
				name == "" && shortName == "" {
				return nullValue(), true, true
			}
		}
		if value, handled, ok := e.structureUnmaterializedEndName(element, property); handled {
			return value, true, ok
		}
		if element.Symbol == nil && !element.IsMembership &&
			(property == "name" || property == "shortName") {
			if target, handled, ok := e.structureNamingFeature(element); handled {
				if !ok {
					reason := Reason{
						Category: ReasonIncompleteStructure,
						Detail:   "unnamed feature's naming feature is unavailable",
					}
					if e.structureNamingReferenceUnresolved(element) {
						reason = Reason{
							Category: ReasonUnresolvedReference,
							Detail:   "Element::" + property + " naming reference target is unresolved",
						}
					}
					e.recordUnsupportedReason(element, "Element", property, reason)
					return Value{}, true, false
				}
				if target.Symbol != nil || target.Node != nil || target.IsMembership || target.Aspect != "" {
					return e.structureNameWithSeen(target, effectiveProperty, seen)
				}
				return nullValue(), true, true
			}
		}
	}
	if element.Symbol != nil && !element.IsMembership {
		isFeature, known := e.isMetaclass(element, "Feature")
		if !known {
			return Value{}, true, false
		}
		if element.Node != nil && element.Node != element.Symbol.Decl {
			return nullValue(), true, true
		}
		if !isFeature {
			if element.Node != nil && element.Node != element.Symbol.Decl {
				return nullValue(), true, true
			}
			if _, alias := element.Symbol.Decl.(*ast.Alias); alias {
				return nullValue(), true, true
			}
			name, shortName, known := symbolDeclaredNames(element.Symbol)
			if !known {
				return nullValue(), true, true
			}
			switch property {
			case "name":
				if name != "" {
					return stringValue(name), true, true
				}
			case "shortName":
				if shortName != "" {
					return stringValue(shortName), true, true
				}
			}
		} else {
			switch property {
			case "name":
				name, shortName, declared := symbolDeclaredNames(element.Symbol)
				if declared && name != "" {
					return stringValue(name), true, true
				}
				if declared && shortName != "" {
					return nullValue(), true, true
				}
				if declared && element.Symbol.Naming == symbols.NamedByDeclaration {
					return nullValue(), true, true
				}
				if target, handled, ok := e.structureNamingFeature(element); handled {
					if !ok {
						e.recordUnresolvedNamingSource(element, property)
						return Value{}, true, false
					}
					if target.Symbol != nil || target.Node != nil || target.IsMembership {
						return e.structureNameWithSeen(target, effectiveProperty, seen)
					}
					if element.Symbol.Naming == symbols.NamedByRedefinition &&
						!e.symbolNamingSourceResolved(element.Symbol) {
						e.recordUnresolvedNamingSource(element, property)
						return Value{}, true, false
					}
					return nullValue(), true, true
				}
				if e.model != nil {
					if name := e.model.EffectiveNameOf(element.Symbol); name != "" {
						return stringValue(name), true, true
					}
				}
			case "shortName":
				_, shortName, declared := symbolDeclaredNames(element.Symbol)
				if declared && shortName != "" {
					return stringValue(shortName), true, true
				}
				if target, handled, ok := e.structureNamingFeature(element); handled {
					if !ok {
						e.recordUnresolvedNamingSource(element, property)
						return Value{}, true, false
					}
					if target.Symbol != nil || target.Node != nil || target.IsMembership {
						return e.structureNameWithSeen(target, effectiveProperty, seen)
					}
					if element.Symbol.Naming == symbols.NamedByRedefinition &&
						!e.symbolNamingSourceResolved(element.Symbol) {
						e.recordUnresolvedNamingSource(element, property)
						return Value{}, true, false
					}
					return nullValue(), true, true
				}
				if element.Symbol.Naming == symbols.NamedByRedefinition &&
					!e.symbolNamingSourceResolved(element.Symbol) {
					e.recordUnresolvedNamingSource(element, property)
					return Value{}, true, false
				}
				if e.model != nil {
					if shortName := e.model.EffectiveShortNameOf(element.Symbol); shortName != "" {
						return stringValue(shortName), true, true
					}
				}
			}
			if e.options.Structure == nil && element.Symbol.Naming == symbols.NamedByRedefinition {
				if targets := e.model.RedefinedFeatures(element.Symbol); len(targets) > 0 && targets[0] != nil {
					return e.structureNameWithSeen(ElementOf(targets[0]), effectiveProperty, seen)
				}
				if !e.symbolNamingSourceResolved(element.Symbol) {
					e.recordUnsupportedReason(
						element, "Element", property,
						Reason{
							Category: ReasonUnresolvedReference,
							Detail:   "Element::" + property + " naming redefinition target is unresolved",
						},
					)
					return Value{}, true, false
				}
			}
			if e.options.Structure == nil && element.Symbol.Naming == symbols.NamedByReference {
				if target := e.model.NamingFeature(element.Symbol); target != nil {
					return e.structureNameWithSeen(ElementOf(target), effectiveProperty, seen)
				}
			}
		}
	}
	if element.Symbol == nil && !element.IsMembership &&
		(property == "name" || property == "shortName") && e.options.Structure != nil {
		isFeature, known := e.isMetaclass(element, "Feature")
		if !known {
			return Value{}, true, false
		}
		if isFeature {
			isPayloadFeature, known := e.isMetaclass(element, "PayloadFeature")
			if !known {
				return Value{}, true, false
			}
			isFeature = !isPayloadFeature
		}
		if isFeature {
			relationships, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
			if !handled || !ok {
				e.recordUnsupportedReason(element, "Element", property, Reason{
					Category: ReasonMissingStructure,
					Detail:   "Element::" + property + " cannot inspect owned Redefinition relationships",
				})
				return Value{}, true, false
			}
			for _, relationship := range relationships.Values {
				if relationship.Kind != ElementValue {
					e.recordUnsupportedReason(element, "Element", property, Reason{
						Category: ReasonUnrepresentedEndpoint,
						Detail:   "Element::" + property + " has a non-element Redefinition relationship",
					})
					return Value{}, true, false
				}
				targets, ok := e.relationshipEndpoints(relationship.Element, "redefinedFeature")
				if !ok {
					category := ReasonUnrepresentedEndpoint
					detail := "Element::" + property + " naming Redefinition::redefinedFeature target is unrepresented"
					if e.hasUnresolvedRelatedElement(relationship.Element, "redefinedFeature") {
						category = ReasonUnresolvedReference
						detail = "Element::" + property + " naming Redefinition::redefinedFeature target is unresolved"
					} else if e.hasStatedRelatedElement(relationship.Element, "redefinedFeature") {
						category = ReasonMissingStructure
						detail = "Element::" + property + " naming Redefinition::redefinedFeature target has no facts in Structure"
					}
					e.recordUnsupportedReason(element, "Element", property, Reason{
						Category: category,
						Detail:   detail,
					})
					return Value{}, true, false
				}
				if len(targets) == 0 {
					continue
				}
				if targets[0].Kind != ElementValue {
					e.recordUnsupportedReason(element, "Element", property, Reason{
						Category: ReasonUnrepresentedEndpoint,
						Detail:   "Element::" + property + " Redefinition target is not an element",
					})
					return Value{}, true, false
				}
				return e.structureNameWithSeen(targets[0].Element, property, seen)
			}
			return nullValue(), true, true
		}
	}
	if e.options.Structure == nil {
		return nullValue(), true, true
	}
	if property == "name" || property == "shortName" {
		isPayloadFeature, known := e.isMetaclass(element, "PayloadFeature")
		if !known {
			return Value{}, true, false
		}
		if isPayloadFeature {
			redefinitions, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
			if !handled || !ok {
				e.recordUnsupportedReason(element, "Element", effectiveProperty, Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "Element::" + effectiveProperty + " cannot inspect owned Redefinition relationships",
				})
				return Value{}, true, false
			}
			if len(redefinitions.Values) == 0 {
				if name, ok := e.structurePayloadFeatureName(effectiveProperty); ok {
					return name, true, true
				}
				e.recordUnsupportedReason(element, "Element", effectiveProperty, Reason{
					Category: ReasonImpliedRelationshipNotMaterialized,
					Detail:   "PayloadFeature::" + effectiveProperty + " requires the implied Redefinition of Transfers::Transfer::payload",
				})
				return Value{}, true, false
			}
		}
	}
	if !element.IsMembership {
		isFeature, known := e.isMetaclass(element, "Feature")
		if !known {
			return Value{}, true, false
		}
		if isFeature {
			for _, naming := range []struct {
				metaclass string
				endpoint  string
			}{
				{metaclass: "Redefinition", endpoint: "redefinedFeature"},
			} {
				relationships, handled, ok := e.structureOwnedFilter(element, naming.metaclass, false)
				if !handled || !ok {
					e.recordUnsupportedReason(
						element, "Element", effectiveProperty,
						Reason{
							Category: ReasonUnrepresentedEndpoint,
							Detail:   "Element::" + effectiveProperty + " cannot inspect owned Redefinition relationships",
						},
					)
					return Value{}, true, false
				}
				for _, relationship := range relationships.Values {
					if relationship.Kind != ElementValue {
						e.recordUnsupportedReason(
							element, "Element", effectiveProperty,
							Reason{
								Category: ReasonUnrepresentedEndpoint,
								Detail:   "Element::" + effectiveProperty + " has a non-element Redefinition relationship",
							},
						)
						return Value{}, true, false
					}
					features, ok := e.relationshipEndpoints(relationship.Element, naming.endpoint)
					if !ok {
						category := ReasonUnrepresentedEndpoint
						detail := "Element::" + effectiveProperty + " naming Redefinition::" + naming.endpoint + " target is unrepresented"
						if e.hasUnresolvedRelatedElement(relationship.Element, naming.endpoint) {
							category = ReasonUnresolvedReference
							detail = "Element::" + effectiveProperty + " naming Redefinition::" + naming.endpoint + " target is unresolved"
						} else if e.hasStatedRelatedElement(relationship.Element, naming.endpoint) {
							category = ReasonMissingStructure
							detail = "Element::" + effectiveProperty + " naming Redefinition::" + naming.endpoint + " target has no facts in Structure"
						}
						e.recordUnsupportedReason(element, "Element", effectiveProperty, Reason{
							Category: category,
							Detail:   detail,
						})
						return Value{}, true, false
					}
					if len(features) == 0 {
						continue
					}
					if features[0].Kind != ElementValue {
						e.recordUnsupportedReason(
							element, "Element", effectiveProperty,
							Reason{
								Category: ReasonUnrepresentedEndpoint,
								Detail:   "Element::" + effectiveProperty + " naming Redefinition::" + naming.endpoint + " is not an element",
							},
						)
						return Value{}, true, false
					}
					value, handled, ok := e.structureNameWithSeen(
						features[0].Element, effectiveProperty, seen,
					)
					if handled && !ok {
						reason := Reason{
							Category:   ReasonDependsOnUnsupported,
							Dependency: "Element::" + effectiveProperty,
							Detail:     "naming redefinition target",
						}
						if targetReason, explained := e.Explain(
							features[0].Element, "Element", effectiveProperty,
						); explained {
							reason = targetReason
						}
						e.recordUnsupportedReason(
							element, "Element", effectiveProperty,
							reason,
						)
					}
					return value, handled, ok
				}
			}
			if element.Symbol != nil && element.Symbol.Naming == symbols.NamedByRedefinition &&
				e.model != nil && e.model.NamingFeature(element.Symbol) == nil {
				e.recordUnresolvedNamingSource(element, property)
				return Value{}, true, false
			}
			return nullValue(), true, true
		}
	}
	return nullValue(), true, true
}

func (e *Evaluator) recordUnresolvedNamingSource(element Element, property string) {
	if element.Symbol == nil || element.Symbol.Naming != symbols.NamedByRedefinition ||
		e.model == nil || e.model.NamingFeature(element.Symbol) != nil {
		return
	}
	e.recordUnsupportedReason(
		element, "Element", property,
		Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "Element::" + property + " naming redefinition target is unresolved",
		},
	)
}

func symbolDeclaredNames(symbol *symbols.Symbol) (string, string, bool) {
	if symbol == nil {
		return "", "", false
	}
	name, shortName, known := astDeclaredNames(symbol.Decl)
	if known {
		return name, shortName, true
	}
	if symbol.Naming == symbols.NamedByDeclaration {
		return symbol.Name, symbol.ShortName, true
	}
	return name, shortName, known
}

func structureDeclaredNames(element Element) (string, string, bool) {
	if element.Aspect != "" {
		return "", "", false
	}
	if element.Symbol != nil {
		if element.Node != nil && element.Node != element.Symbol.Decl {
			return "", "", false
		}
		return symbolDeclaredNames(element.Symbol)
	}
	return astDeclaredNames(element.Node)
}

func (e *Evaluator) structureDeclaredIdentifiers(element Element) (string, string, bool) {
	name, shortName, known := structureDeclaredNames(element)
	if known && (name != "" || shortName != "") {
		return name, shortName, true
	}
	if e.options.Structure == nil || element.IsMembership {
		return name, shortName, false
	}
	if value, ok := e.options.Structure.Attribute(element, "declaredName"); ok &&
		value.Kind == StringValue && value.String != "" {
		name = value.String
	}
	if value, ok := e.options.Structure.Attribute(element, "declaredShortName"); ok &&
		value.Kind == StringValue && value.String != "" {
		shortName = value.String
	}
	return name, shortName, name != "" || shortName != ""
}

func astDeclaredNames(node ast.Node) (string, string, bool) {
	switch declaration := node.(type) {
	case *ast.Usage:
		return declaration.Ident.Name, declaration.Ident.ShortName, true
	case *ast.Definition:
		return declaration.Ident.Name, declaration.Ident.ShortName, true
	case *ast.Package:
		return declaration.Ident.Name, declaration.Ident.ShortName, true
	case *ast.SubjectMember:
		return declaration.Ident.Name, declaration.Ident.ShortName, true
	case *ast.CrossFeatureMember:
		return declaration.Ident.Name, declaration.Ident.ShortName, true
	case *ast.ConnectorEnd:
		identification, ok := declaration.DeclaredName()
		if !ok {
			return "", "", true
		}
		return identification.Name, identification.ShortName, true
	case *ast.AssumeMember, *ast.RequireMember:
		if constraint, ok := ast.OwnedConstraintOf(declaration); ok {
			return constraint.Ident.Name, constraint.Ident.ShortName, true
		}
		return "", "", true
	case *ast.InitialNode:
		if declaration.Successor != nil {
			return "", "", true
		}
		return declaration.Name(), "", true
	case *ast.SendStatement, *ast.SuccessionEdge, *ast.ControlFlowEdge:
		return "", "", true
	case *ast.PseudostateNode:
		return declaration.Name, "", true
	case *ast.SubstateMember:
		return declaration.Name, "", true
	case *ast.TransitionMember:
		return declaration.Name, "", true
	default:
		return "", "", false
	}
}

func (e *Evaluator) structureNamingFeature(element Element) (Element, bool, bool) {
	symbol := element.Symbol
	if symbol == nil || e.model == nil {
		if e.options.Structure == nil {
			return Element{}, true, false
		}
		isFeature, known := e.isMetaclass(element, "Feature")
		if !known {
			return Element{}, true, false
		}
		if !isFeature {
			return Element{}, false, false
		}
		if target, handled, ok := e.structureReferenceNamingFeature(element); handled {
			if !ok {
				return Element{}, true, false
			}
			return target, true, true
		}
		redefinitions, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
		if !handled || !ok {
			return Element{}, true, false
		}
		if len(redefinitions.Values) == 0 {
			return Element{}, false, true
		}
		redefinition := redefinitions.Values[0]
		if redefinition.Kind != ElementValue {
			return Element{}, true, false
		}
		targets, ok := e.relationshipEndpoints(redefinition.Element, "redefinedFeature")
		if !ok || len(targets) == 0 || targets[0].Kind != ElementValue {
			return Element{}, true, false
		}
		isFeatureChain, known := e.isMetaclass(targets[0].Element, "FeatureChainExpression")
		if !known {
			return Element{}, true, false
		}
		if isFeatureChain {
			return Element{}, true, true
		}
		return targets[0].Element, true, true
	}
	isPerformAction, known := e.isMetaclass(element, "PerformActionUsage")
	if !known {
		return Element{}, true, false
	}
	if isPerformAction {
		value, handled, ok := e.structurePerformedAction(element)
		if !handled || !ok {
			if e.model.ReferencedFeature(symbol) == nil && symbolNamingReferenceUnresolved(symbol) {
				return Element{}, true, true
			}
			return Element{}, true, false
		}
		if value.Kind == ElementValue {
			return value.Element, true, true
		}
		return Element{}, true, true
	}
	membershipKind := ""
	membership, handled, ok := e.structureOwningMembership(element)
	if handled && ok {
		switch membership.Kind {
		case ElementValue:
			membershipKind, known = e.options.Structure.Metaclass(membership.Element)
			if !known {
				return Element{}, true, false
			}
		case MembershipValue:
			membershipElement := MembershipElement(membership.Membership)
			membershipKind, known = e.options.Structure.Metaclass(membershipElement)
			if !known {
				membershipKind = membership.Membership.Metaclass()
			}
		case NullValue:
			membershipKind = element.Membership.Metaclass()
		default:
			return Element{}, true, false
		}
	}
	if membershipKind == "" {
		membershipKind = element.Membership.Metaclass()
	}
	if membershipKind == "VariantMembership" {
		target, handled, ok := e.structureReferencedFeature(element)
		if !handled || !ok {
			return Element{}, true, false
		}
		if target.Symbol != nil || target.Node != nil || target.IsMembership || target.Aspect != "" {
			return target, true, true
		}
		if target := e.model.ReferencedFeature(symbol); target != nil {
			return ElementOf(target), true, true
		}
		return Element{}, true, false
	}
	isConstraint, known := e.isMetaclass(element, "ConstraintUsage")
	if !known {
		return Element{}, true, false
	}
	if isConstraint && membershipKind == "RequirementConstraintMembership" {
		referenced, handled, ok := e.structureReferencedFeature(element)
		if !handled || !ok {
			return Element{}, true, false
		}
		if referenced.Symbol == nil && referenced.Node == nil && !referenced.IsMembership && referenced.Aspect == "" {
			if target := e.model.ReferencedFeature(symbol); target != nil {
				referenced = ElementOf(target)
			} else {
				return Element{}, true, false
			}
		}
		target, ok := e.Property(referenced, "Feature", "featureTarget")
		if !ok {
			return Element{}, true, false
		}
		if target.Kind != ElementValue {
			return Element{}, true, true
		}
		return target.Element, true, true
	}
	isReferenceUsage, known := e.isMetaclass(element, "ReferenceUsage")
	if !known {
		return Element{}, true, false
	}
	if isReferenceUsage {
		owningType, handled, ok := e.structureOwningType(element)
		if handled && ok && owningType.Kind == ElementValue {
			isTransition, known := e.isMetaclass(owningType.Element, "TransitionUsage")
			if !known {
				return Element{}, true, false
			}
			if isTransition {
				inputs, ok := e.Property(owningType.Element, "Type", "input")
				if !ok || inputs.Kind != SequenceValue {
					return Element{}, true, false
				}
				if len(inputs.Values) >= 2 && inputs.Values[1].Kind == ElementValue &&
					sameStructureElement(inputs.Values[1].Element, element, e.options.Structure) {
					actions, ok := e.Property(owningType.Element, "TransitionUsage", "triggerAction")
					if !ok || actions.Kind != SequenceValue {
						return Element{}, true, false
					}
					if len(actions.Values) == 0 || actions.Values[0].Kind != ElementValue {
						return Element{}, true, true
					}
					payload, ok := e.Property(actions.Values[0].Element, "AcceptActionUsage", "payloadParameter")
					if !ok {
						return Element{}, true, false
					}
					if payload.Kind != ElementValue {
						return Element{}, true, true
					}
					return payload.Element, true, true
				}
			}
		}
	}
	if e.options.Structure != nil {
		isFeature, known := e.isMetaclass(element, "Feature")
		if !known {
			return Element{}, true, false
		}
		if !isFeature {
			return Element{}, false, false
		}
		relationships, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
		if !handled || !ok {
			return Element{}, true, false
		}
		for _, relationship := range relationships.Values {
			if relationship.Kind != ElementValue {
				return Element{}, true, false
			}
			features, ok := e.relationshipEndpoints(relationship.Element, "redefinedFeature")
			if !ok {
				return Element{}, true, false
			}
			if len(features) == 0 {
				continue
			}
			if features[0].Kind != ElementValue {
				return Element{}, true, false
			}
			return features[0].Element, true, true
		}
		if symbol.Naming == symbols.NamedByRedefinition &&
			!e.symbolNamingSourceResolved(symbol) {
			return Element{}, true, false
		}
		if target := e.model.NamingFeature(symbol); target != nil {
			return ElementOf(target), true, true
		}
		isPayloadFeature, known := e.isMetaclass(element, "PayloadFeature")
		if !known {
			return Element{}, true, false
		}
		if isPayloadFeature {
			if target, ok := e.structurePayloadFeatureNamingTarget(); ok {
				return target, true, true
			}
			return Element{}, false, false
		}
		return Element{}, true, true
	}
	if symbol.Naming == symbols.NamedByRedefinition && !e.symbolNamingSourceResolved(symbol) {
		return Element{}, true, false
	}
	if target := e.model.NamingFeature(symbol); target != nil {
		return ElementOf(target), true, true
	}
	switch symbol.Naming {
	case symbols.NamedByRedefinition:
		return Element{}, true, e.symbolNamingSourceResolved(symbol)
	case symbols.NamedByReference:
		return Element{}, true, true
	default:
		return Element{}, true, true
	}
}

func (e *Evaluator) structureReferenceNamingFeature(element Element) (Element, bool, bool) {
	for _, metaclass := range []string{
		"PerformActionUsage", "ExhibitStateUsage", "IncludeUseCaseUsage",
		"RenderingUsage",
	} {
		named, known := e.isMetaclass(element, metaclass)
		if !known {
			return Element{}, true, false
		}
		if named {
			target, handled, ok := e.structureReferencedFeature(element)
			if !handled || !ok {
				return Element{}, true, false
			}
			return e.structureReferenceNamingTarget(target)
		}
	}

	membership, handled, ok := e.structureOwningMembership(element)
	if !handled || !ok {
		return Element{}, true, false
	}
	membershipClass := ""
	switch membership.Kind {
	case ElementValue:
		membershipClass, ok = e.options.Structure.Metaclass(membership.Element)
		if !ok {
			return Element{}, true, false
		}
	case MembershipValue:
		membershipClass = membership.Membership.Metaclass()
		if membershipClass == "" {
			membershipClass, ok = e.options.Structure.Metaclass(MembershipElement(membership.Membership))
			if !ok {
				return Element{}, true, false
			}
		}
	case NullValue:
		membershipClass = element.Membership.Metaclass()
	default:
		return Element{}, true, false
	}
	switch membershipClass {
	case "FramedConcernMembership", "RequirementConstraintMembership",
		"RequirementVerificationMembership", "StateSubactionMembership", "VariantMembership",
		"ViewRenderingMembership":
		target, handled, ok := e.structureReferencedFeature(element)
		if !handled || !ok {
			return Element{}, true, false
		}
		return e.structureReferenceNamingTarget(target)
	default:
		return Element{}, false, true
	}
}

func (e *Evaluator) structureNamingReferenceUnresolved(element Element) bool {
	references, handled, ok := e.structureOwnedFilter(element, "ReferenceSubsetting", false)
	if !handled || !ok {
		return false
	}
	for _, reference := range references.Values {
		if reference.Kind != ElementValue {
			continue
		}
		targets, represented := e.relationshipEndpoints(reference.Element, "referencedFeature")
		if represented && len(targets) == 1 && targets[0].Kind == ElementValue {
			continue
		}
		if e.hasUnresolvedRelatedElement(reference.Element, "referencedFeature") ||
			e.hasStatedRelatedElement(reference.Element, "referencedFeature") {
			return true
		}
	}
	return false
}

func (e *Evaluator) structureReferenceNamingTarget(target Element) (Element, bool, bool) {
	if target.Symbol == nil && target.Node == nil && !target.IsMembership && target.Aspect == "" {
		return Element{}, false, true
	}
	isChain, known := e.isMetaclass(target, "FeatureChainExpression")
	if !known {
		return Element{}, true, false
	}
	if !isChain {
		return target, true, true
	}
	chains, handled, ok := e.structureOwnedFilter(target, "FeatureChaining", false)
	if !handled || !ok {
		return Element{}, true, false
	}
	if len(chains.Values) == 0 {
		return Element{}, true, true
	}
	last := chains.Values[len(chains.Values)-1]
	if last.Kind != ElementValue {
		return Element{}, true, false
	}
	features, ok := e.relationshipEndpoints(last.Element, "chainingFeature")
	if !ok || len(features) == 0 || features[len(features)-1].Kind != ElementValue {
		return Element{}, true, false
	}
	return features[len(features)-1].Element, true, true
}

func (e *Evaluator) structurePayloadFeatureNamingTarget() (Element, bool) {
	if e.resolver == nil || e.resolver.Index() == nil {
		return Element{}, false
	}
	index := e.resolver.Index()
	catalog := identity.LibraryCatalog(index)
	var target *symbols.Symbol
	for _, candidate := range index.LookupQualified("Transfers::Transfer::payload") {
		if _, library := catalog.ElementForSymbol(candidate); !library {
			continue
		}
		if target != nil {
			return Element{}, false
		}
		target = candidate
	}
	if target == nil {
		return Element{}, false
	}
	return ElementOf(target), true
}

func (e *Evaluator) structurePayloadFeatureName(property string) (Value, bool) {
	target, ok := e.structurePayloadFeatureNamingTarget()
	if !ok || target.Symbol == nil {
		return Value{}, false
	}
	name := target.Symbol.Name
	if e.model != nil {
		name = e.model.EffectiveNameOf(target.Symbol)
	}
	if property == "shortName" {
		name = target.Symbol.ShortName
		if e.model != nil {
			name = e.model.EffectiveShortNameOf(target.Symbol)
		}
	}
	if name == "" {
		return nullValue(), true
	}
	return stringValue(name), true
}

func symbolNamingReferenceUnresolved(symbol *symbols.Symbol) bool {
	return symbol != nil && symbol.Naming == symbols.NamedByReference && symbol.NamingTarget != nil
}

func (e *Evaluator) symbolNamingSourceResolved(symbol *symbols.Symbol) bool {
	if symbol == nil || symbol.Naming == symbols.NamedByDeclaration {
		return true
	}
	if e.model == nil {
		return false
	}
	switch symbol.Naming {
	case symbols.NamedByReference:
		return e.model.ReferencedFeature(symbol) != nil
	case symbols.NamedByRedefinition:
		return len(e.model.RedefinedFeatures(symbol)) > 0
	default:
		return false
	}
}

func (e *Evaluator) structureAnnotatedElements(element Element) (Value, bool, bool) {
	if comment, ok := element.Node.(*ast.Comment); ok && len(comment.About) > 0 {
		value, resolved := e.structureAboutTargets(element, comment.About)
		return value, true, resolved
	}
	if metadata, ok := element.Node.(*ast.PrefixMetadata); ok && len(metadata.About) > 0 {
		value, resolved := e.structureAboutTargets(element, metadata.About)
		return value, true, resolved
	}
	if usage, ok := element.Node.(*ast.Usage); ok && symbols.AnnotatesOthers(usage) {
		about := symbols.MetadataAboutRefs(usage)
		if len(about) == 0 {
			return Value{}, true, false
		}
		value, resolved := e.structureAboutTargets(element, about)
		return value, true, resolved
	}
	if e.hasStatedRelatedElement(element, "annotatedElement") {
		targets, ok := e.statedRelatedElements(element, "annotatedElement")
		if !ok {
			return Value{}, true, false
		}
		values := make([]Value, 0, len(targets))
		for _, target := range targets {
			values = append(values, elementValue(target))
		}
		return uniqueStructureValueSequence(values, e.options.Structure), true, true
	}
	annotations, handled, ok := e.structureAnnotation(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(annotations.Values))
	for _, annotation := range annotations.Values {
		if annotation.Kind != ElementValue {
			return Value{}, true, false
		}
		targets, ok := e.relationshipEndpoints(annotation.Element, "annotatedElement")
		if !ok || len(targets) == 0 {
			return Value{}, true, false
		}
		values = append(values, targets...)
	}
	if len(annotations.Values) > 0 {
		return uniqueStructureValueSequence(values, e.options.Structure), true, true
	}
	namespace, handled, ok := e.structureOwningNamespace(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	return sequence(namespace), true, true
}

func (e *Evaluator) structureAboutTargets(element Element, about []*ast.QualifiedName) (Value, bool) {
	if e.resolver == nil {
		return Value{}, false
	}
	var scope *symbols.Scope
	if element.Symbol != nil {
		scope = element.Symbol.OwnerScope
	}
	if scope == nil && element.Container != nil {
		scope = element.Container.Scope
	}
	if scope == nil {
		return Value{}, false
	}
	values := make([]Value, 0, len(about))
	for _, name := range about {
		target, ok := e.resolver.ResolveQualified(scope, name)
		if !ok || target == nil {
			return Value{}, false
		}
		values = append(values, elementValue(ElementOf(target)))
	}
	return sequence(values...), true
}

func (e *Evaluator) structureAnnotation(element Element) (Value, bool, bool) {
	if comment, ok := element.Node.(*ast.Comment); ok && len(comment.About) > 0 {
		return Value{}, true, false
	}
	owned, handled, ok := e.structureOwnedAnnotatingRelationships(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	owning, handled, ok := e.structureOwningAnnotatingRelationship(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if owning.Kind != ElementValue {
		return owned, true, true
	}
	values := make([]Value, 0, len(owned.Values)+1)
	values = append(values, owning)
	values = append(values, owned.Values...)
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
}

func (e *Evaluator) structureOwnedAnnotatingRelationships(element Element) (Value, bool, bool) {
	if comment, ok := element.Node.(*ast.Comment); ok && len(comment.About) > 0 {
		return Value{}, true, false
	}
	owned, handled, ok := e.structureOwnedFilter(element, "Annotation", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(owned.Values))
	for _, annotation := range owned.Values {
		if annotation.Kind != ElementValue {
			return Value{}, true, false
		}
		targets, ok := e.relationshipEndpoints(annotation.Element, "annotatedElement")
		if !ok || len(targets) == 0 {
			return Value{}, true, false
		}
		annotatesSelf := false
		for _, target := range targets {
			if target.Kind != ElementValue {
				return Value{}, true, false
			}
			if sameStructureElement(target.Element, element, e.options.Structure) {
				annotatesSelf = true
				break
			}
		}
		if !annotatesSelf {
			values = append(values, annotation)
		}
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureOwningAnnotatingRelationship(element Element) (Value, bool, bool) {
	var relationship Element
	var present, known bool
	if structure, ok := e.options.Structure.(owningAnnotatingRelationshipStructure); ok {
		relationship, present, known = structure.OwningAnnotatingRelationship(element)
	} else {
		relationship, present, known = e.options.Structure.OwningRelationship(element)
	}
	if !known {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	matches, known := e.isMetaclass(relationship, "Annotation")
	if !known {
		return Value{}, true, false
	}
	if !matches {
		return nullValue(), true, true
	}
	return elementValue(relationship), true, true
}

func (e *Evaluator) structureAnnotationOwnedAnnotatingElement(annotation Element) (Value, bool, bool) {
	related, ok := e.options.Structure.OwnedRelatedElements(annotation)
	if !ok {
		return Value{}, true, false
	}
	filtered, ok := e.structureFilterValues(structureSequence(related).Values, "AnnotatingElement")
	if !ok {
		return Value{}, true, false
	}
	if len(filtered) == 0 {
		return nullValue(), true, true
	}
	return filtered[0], true, true
}

func (e *Evaluator) structureAnnotationOwningAnnotatingElement(annotation Element) (Value, bool, bool) {
	related, present, ok := e.options.Structure.OwningRelatedElement(annotation)
	if !ok {
		return Value{}, true, false
	}
	if !present {
		return nullValue(), true, true
	}
	filtered, ok := e.structureFilterValues([]Value{elementValue(related)}, "AnnotatingElement")
	if !ok {
		return Value{}, true, false
	}
	if len(filtered) == 0 {
		return nullValue(), true, true
	}
	if len(filtered) > 1 {
		return Value{}, true, false
	}
	return filtered[0], true, true
}

func (e *Evaluator) structureAnnotationAnnotatingElement(annotation Element) (Value, bool, bool) {
	owned, handled, ok := e.structureAnnotationOwnedAnnotatingElement(annotation)
	if !handled || !ok {
		return Value{}, true, false
	}
	if owned.Kind == ElementValue {
		return owned, true, true
	}
	return e.structureAnnotationOwningAnnotatingElement(annotation)
}

type structureQualifiedNameCacheEntry struct {
	value   Value
	handled bool
	ok      bool
}

type structureQualifiedNameSiblingNames struct {
	computed  bool
	supported bool
	members   []Element
	reason    Reason
}

type structureQualifiedNameNamespaceCache struct {
	ownedMembers     structureQualifiedNameSiblingNames
	ownedMemberships structureQualifiedNameSiblingNames
	ownerComputed    bool
	ownerValue       Value
	ownerHandled     bool
	ownerOK          bool
}

func (e *Evaluator) structureQualifiedName(element Element, seen map[ElementKey]bool) (Value, bool, bool) {
	key := element.Key()
	e.mu.Lock()
	cached, known := e.structureQualifiedNames[key]
	e.mu.Unlock()
	if known {
		return cloneValue(cached.value), cached.handled, cached.ok
	}
	value, handled, ok := e.structureQualifiedNameBase(element, seen)
	if !handled {
		return value, false, ok
	}
	e.mu.Lock()
	if e.structureQualifiedNames == nil {
		e.structureQualifiedNames = make(map[ElementKey]structureQualifiedNameCacheEntry)
	}
	if cached, known = e.structureQualifiedNames[key]; known {
		value, handled, ok = cloneValue(cached.value), cached.handled, cached.ok
	} else {
		e.structureQualifiedNames[key] = structureQualifiedNameCacheEntry{
			value: cloneValue(value), handled: handled, ok: ok,
		}
	}
	e.mu.Unlock()
	return value, handled, ok
}

func (e *Evaluator) structureQualifiedNameBase(element Element, seen map[ElementKey]bool) (Value, bool, bool) {
	key := element.Key()
	e.mu.Lock()
	cached, known := e.structureQualifiedNameBases[key]
	e.mu.Unlock()
	if known {
		return cloneValue(cached.value), cached.handled, cached.ok
	}
	if seen[key] {
		e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "owning namespace qualified-name cycle",
		})
		return Value{}, true, false
	}
	seen[key] = true
	value, handled, ok := e.deriveStructureQualifiedNameBase(element, seen)
	delete(seen, key)
	if !handled {
		return value, false, ok
	}
	e.mu.Lock()
	if e.structureQualifiedNameBases == nil {
		e.structureQualifiedNameBases = make(map[ElementKey]structureQualifiedNameCacheEntry)
	}
	if cached, known = e.structureQualifiedNameBases[key]; known {
		value, handled, ok = cloneValue(cached.value), cached.handled, cached.ok
	} else {
		e.structureQualifiedNameBases[key] = structureQualifiedNameCacheEntry{
			value: cloneValue(value), handled: handled, ok: ok,
		}
	}
	e.mu.Unlock()
	return value, handled, ok
}

func (e *Evaluator) deriveStructureQualifiedNameBase(element Element, seen map[ElementKey]bool) (Value, bool, bool) {
	if structure, ok := e.options.Structure.(QualifiedNameStructure); ok {
		if qualifiedName, known := structure.QualifiedName(element); known {
			return stringValue(qualifiedName), true, true
		}
	}
	var name Value
	name, handled, ok := e.structureName(element, "name")
	if !handled || !ok {
		reason := Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Element::name",
		}
		if explanation, explained := e.Explain(element, "Element", "name"); explained {
			reason = explanation
		}
		e.recordUnsupportedReason(element, "Element", "qualifiedName", reason)
		return Value{}, true, false
	}
	if name.Kind == NullValue {
		shortName, handled, ok := e.structureName(element, "shortName")
		if !handled || !ok {
			reason := Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Element::shortName",
			}
			if explanation, explained := e.Explain(element, "Element", "shortName"); explained {
				reason = explanation
			}
			e.recordUnsupportedReason(element, "Element", "qualifiedName", reason)
			return Value{}, true, false
		}
		if shortName.Kind == NullValue {
			return nullValue(), true, true
		}
		if shortName.Kind != StringValue {
			e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Element::shortName has an unsupported value",
			})
			return Value{}, true, false
		}
	}
	if name.Kind != StringValue && name.Kind != NullValue {
		e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
			Category: ReasonUnrepresentedElement,
			Detail:   "Element::name has an unsupported value",
		})
		return Value{}, true, false
	}
	namespace, handled, ok := e.structureOwningNamespace(element)
	if !handled || !ok {
		reason := Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Element::owningNamespace",
		}
		if e.hasUnresolvedRelatedElement(element, "owningRelationship") {
			reason = Reason{
				Category: ReasonMissingStructure,
				Detail:   "qualified name's owning relationship is not represented in the graph",
			}
		}
		e.recordUnsupportedReason(element, "Element", "qualifiedName", reason)
		return Value{}, true, false
	}
	if namespace.Kind != ElementValue {
		return nullValue(), true, true
	}
	escapedName := nullValue()
	if name.Kind == StringValue {
		escapedName = stringValue(identity.EscapeName(name.String))
		duplicate, reason, supported := e.structureQualifiedNameHasEarlierName(
			namespace.Element, element, name.String, element.IsMembership,
		)
		if !supported {
			e.recordUnsupportedReason(element, "Element", "qualifiedName", reason)
			return Value{}, true, false
		}
		if duplicate {
			return nullValue(), true, true
		}
	}
	if name.Kind == NullValue {
		shortName, handled, ok := e.structureName(element, "shortName")
		if !handled || !ok {
			e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Element::shortName",
			})
			return Value{}, true, false
		}
		if shortName.Kind != StringValue && shortName.Kind != NullValue {
			e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Element::shortName has an unsupported value",
			})
			return Value{}, true, false
		}
		if shortName.Kind == StringValue {
			escapedName = stringValue(identity.EscapeName(shortName.String))
		}
	}
	namespaceOwner, handled, ok := e.structureQualifiedNameNamespaceOwner(namespace.Element)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Element::owner",
			Detail:     "owning namespace owner is unavailable",
		})
		return Value{}, true, false
	}
	if namespaceOwner.Kind != ElementValue {
		return escapedName, true, true
	}
	if escapedName.Kind == NullValue {
		return nullValue(), true, true
	}
	parent, handled, ok := e.structureQualifiedNameBase(namespace.Element, seen)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Element", "qualifiedName", Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Element::qualifiedName",
		})
		return Value{}, true, false
	}
	if parent.Kind != StringValue {
		return nullValue(), true, true
	}
	return stringValue(parent.String + "::" + escapedName.String), true, true
}

func (e *Evaluator) structureQualifiedNameNamespaceOwner(namespace Element) (Value, bool, bool) {
	key := namespace.Key()
	e.mu.Lock()
	cached := e.qualifiedNameNamespaceCaches[key]
	e.mu.Unlock()
	if cached.ownerComputed {
		return cloneValue(cached.ownerValue), cached.ownerHandled, cached.ownerOK
	}
	value, handled, ok := e.structureOwner(namespace)
	e.mu.Lock()
	if e.qualifiedNameNamespaceCaches == nil {
		e.qualifiedNameNamespaceCaches = make(map[ElementKey]structureQualifiedNameNamespaceCache)
	}
	cached = e.qualifiedNameNamespaceCaches[key]
	if !cached.ownerComputed {
		cached.ownerComputed = true
		cached.ownerValue = cloneValue(value)
		cached.ownerHandled = handled
		cached.ownerOK = ok
		e.qualifiedNameNamespaceCaches[key] = cached
	} else {
		value, handled, ok = cloneValue(cached.ownerValue), cached.ownerHandled, cached.ownerOK
	}
	e.mu.Unlock()
	return value, handled, ok
}

func (e *Evaluator) structureQualifiedNameHasEarlierName(
	namespace Element,
	element Element,
	name string,
	memberships bool,
) (bool, Reason, bool) {
	members, reason, supported := e.structureQualifiedNameMembers(namespace, memberships)
	if !supported {
		return false, reason, false
	}
	targetKey := structureElementKey(element, e.options.Structure)
	for _, member := range members {
		if structureElementKey(member, e.options.Structure) == targetKey {
			return false, Reason{}, true
		}
		memberName, handled, ok := e.structureName(member, "name")
		if !handled || !ok {
			label := e.structureIdentityLabel(member)
			if metaclass, known := e.options.Structure.Metaclass(member); known {
				label = metaclass + " " + label
			}
			if label == "" && member.Symbol != nil {
				label = member.Symbol.Name
			}
			if label == "" {
				label = member.Aspect
			}
			if label == "" {
				label = "unnamed member"
			}
			return false, Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Element::name",
				Detail:     "Namespace::ownedMember for " + label,
			}, false
		}
		switch memberName.Kind {
		case NullValue:
			continue
		case StringValue:
			if memberName.String == name {
				return true, Reason{}, true
			}
		default:
			return false, Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Namespace::ownedMember Element::name has an unsupported value",
			}, false
		}
	}
	return false, Reason{
		Category: ReasonMissingStructure,
		Detail:   "Element is not present in its owning namespace membership order",
	}, false
}

func (e *Evaluator) structureQualifiedNameMembers(
	namespace Element,
	memberships bool,
) ([]Element, Reason, bool) {
	key := namespace.Key()
	e.mu.Lock()
	namespaceCache := e.qualifiedNameNamespaceCaches[key]
	cached := namespaceCache.ownedMembers
	if memberships {
		cached = namespaceCache.ownedMemberships
	}
	e.mu.Unlock()
	if cached.computed {
		return cached.members, cached.reason, cached.supported
	}

	computed := e.deriveStructureQualifiedNameMembers(namespace, memberships)
	e.mu.Lock()
	if e.qualifiedNameNamespaceCaches == nil {
		e.qualifiedNameNamespaceCaches = make(map[ElementKey]structureQualifiedNameNamespaceCache)
	}
	namespaceCache = e.qualifiedNameNamespaceCaches[key]
	cached = namespaceCache.ownedMembers
	if memberships {
		cached = namespaceCache.ownedMemberships
	}
	if !cached.computed {
		cached = computed
		if memberships {
			namespaceCache.ownedMemberships = cached
		} else {
			namespaceCache.ownedMembers = cached
		}
		e.qualifiedNameNamespaceCaches[key] = namespaceCache
	}
	e.mu.Unlock()
	return cached.members, cached.reason, cached.supported
}

func (e *Evaluator) deriveStructureQualifiedNameMembers(
	namespace Element,
	memberships bool,
) structureQualifiedNameSiblingNames {
	result := structureQualifiedNameSiblingNames{computed: true}
	var members []Element
	if memberships {
		ownedMemberships, handled, ok := e.structureOwnedFilter(namespace, "Membership", true)
		if !handled || !ok || ownedMemberships.Kind != SequenceValue {
			result.reason = Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Namespace::ownedMembership",
			}
			return result
		}
		members = make([]Element, 0, len(ownedMemberships.Values))
		for _, membership := range ownedMemberships.Values {
			if membership.Kind != MembershipValue {
				result.reason = Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Namespace::ownedMembership contains a non-membership",
				}
				return result
			}
			members = append(members, MembershipElement(membership.Membership))
		}
	} else {
		ownedMembers, handled, ok := e.structureOwnedMembers(namespace)
		if !handled || !ok || ownedMembers.Kind != SequenceValue {
			result.reason = Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Namespace::ownedMember",
			}
			return result
		}
		members = make([]Element, 0, len(ownedMembers.Values))
		for _, member := range ownedMembers.Values {
			if member.Kind != ElementValue {
				result.reason = Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Namespace::ownedMember contains a non-element",
				}
				return result
			}
			members = append(members, member.Element)
		}
	}
	result.members = members
	result.supported = true
	return result
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
	relationship, present, known := e.options.Structure.OwningRelationship(element)
	if !known {
		return Value{}, true, false
	}
	if !present {
		isRelationship, known := e.isMetaclass(element, "Relationship")
		if !known {
			return Value{}, true, false
		}
		if !isRelationship {
			return nullValue(), true, true
		}
		owner, ownerPresent, ownerKnown := e.options.Structure.OwningRelatedElement(element)
		if !ownerKnown {
			return Value{}, true, false
		}
		if !ownerPresent {
			return nullValue(), true, true
		}
		return elementValue(owner), true, true
	}
	owner, present, known := e.options.Structure.OwningRelatedElement(relationship)
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
	isMembership, known := e.isMetaclass(relationship, "OwningMembership")
	if !known {
		return Value{}, true, false
	}
	if !isMembership {
		return nullValue(), true, true
	}
	if !relationship.IsMembership {
		return elementValue(relationship), true, true
	}
	return membershipValue(relationship.Membership), true, true
}

func (e *Evaluator) structureOwningNamespace(element Element) (Value, bool, bool) {
	membership, handled, ok := e.structureOwningMembership(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var membershipElement Element
	switch membership.Kind {
	case MembershipValue:
		membershipElement = MembershipElement(membership.Membership)
	case ElementValue:
		membershipElement = membership.Element
	default:
		return nullValue(), true, true
	}
	namespaces, known := e.relationshipEndpoints(membershipElement, "membershipOwningNamespace")
	if !known {
		return Value{}, true, false
	}
	if len(namespaces) == 0 {
		return nullValue(), true, true
	}
	if len(namespaces) != 1 || namespaces[0].Kind != ElementValue {
		return Value{}, true, false
	}
	namespace := namespaces[0].Element
	isNamespace, known := e.isMetaclass(namespace, "Namespace")
	if !known {
		return Value{}, true, false
	}
	if !isNamespace {
		return nullValue(), true, true
	}
	return elementValue(namespace), true, true
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

func (e *Evaluator) structureOwnedRelationshipValues(element Element) (Value, bool, bool) {
	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(relationships))
	for _, relationship := range relationships {
		values = append(values, elementValue(relationship))
	}
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
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
	return uniqueStructureValueSequence(structureSequence(values).Values, e.options.Structure), true, true
}

func (e *Evaluator) structureOwnedElementFilter(element Element, expected string) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedElements(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	values, ok := e.structureFilterValues(owned.Values, expected)
	if !ok {
		return Value{}, true, false
	}
	return sequence(values...), true, true
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
		if !ok {
			return Value{}, true, false
		}
		return uniqueStructureValueSequence(sequence.Values, e.options.Structure), true, true
	}
	return uniqueStructureValueSequence(structureSequence(values).Values, e.options.Structure), true, true
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
	return uniqueStructureValueSequence(features, e.options.Structure), true, true
}

func (e *Evaluator) structureOwnedMembers(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "OwningMembership", true)
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
	return uniqueStructureValueSequence(structureSequence(members).Values, e.options.Structure), true, true
}

func (e *Evaluator) structureEffectiveFeatures(element Element) (Value, bool, bool) {
	if element.Symbol != nil && e.model != nil {
		portUsage, known := e.isMetaclass(element, "PortUsage")
		if known && portUsage {
			relationships, known := e.options.Structure.OwnedRelationships(element)
			if !known {
				return Value{}, true, false
			}
			for _, relationship := range relationships {
				isConjugatedTyping, known := e.isMetaclass(relationship, "ConjugatedPortTyping")
				if !known {
					return Value{}, true, false
				}
				if !isConjugatedTyping {
					continue
				}
				definitions, known := e.relationshipEndpoints(relationship, "conjugatedPortDefinition")
				if !known {
					return Value{}, true, false
				}
				definitions, known = e.structureFilterValues(definitions, "ConjugatedPortDefinition")
				if !known || len(definitions) != 1 || definitions[0].Kind != ElementValue {
					return Value{}, true, false
				}
				return e.structureEffectiveFeatures(definitions[0].Element)
			}
			var features []*symbols.Symbol
			for _, feature := range e.model.PortFeatures(element.Symbol) {
				if feature.Symbol == nil {
					return Value{}, true, false
				}
				features = append(features, feature.Symbol)
			}
			if len(features) != 0 {
				return symbolSequence(features), true, true
			}
			types, handled, ok := e.structureFeatureTypes(element)
			if !handled || !ok {
				return Value{}, true, false
			}
			var inherited []Value
			for _, typ := range types.Values {
				if typ.Kind != ElementValue {
					return Value{}, true, false
				}
				features, handled, ok := e.structureEffectiveFeatures(typ.Element)
				if !handled || !ok {
					return Value{}, true, false
				}
				inherited = append(inherited, features.Values...)
			}
			return uniqueStructureValueSequence(inherited, e.options.Structure), true, true
		}
	}
	isConjugatedPortDefinition, known := e.isMetaclass(element, "ConjugatedPortDefinition")
	if !known {
		return Value{}, true, false
	}
	if isConjugatedPortDefinition {
		original, ok := e.Property(element, "ConjugatedPortDefinition", "originalPortDefinition")
		if !ok || original.Kind != ElementValue {
			return Value{}, true, false
		}
		isPortDefinition, known := e.isMetaclass(original.Element, "PortDefinition")
		if !known {
			return Value{}, true, false
		}
		if isPortDefinition && original.Element.Symbol != nil && e.model != nil {
			var features []*symbols.Symbol
			for _, feature := range e.model.PortFeatures(original.Element.Symbol) {
				if feature.Symbol == nil {
					return Value{}, true, false
				}
				features = append(features, feature.Symbol)
			}
			return symbolSequence(features), true, true
		}
		owned, handled, ok := e.structureOwnedFeatures(original.Element)
		if !handled || !ok {
			return Value{}, true, false
		}
		inherited, ok := e.structureInheritedValues(original.Element, "feature")
		if !ok {
			e.recordUnsupportedReason(original.Element, "Type", "feature", Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "ConjugatedPortDefinition::feature has unresolved specialization references",
			})
			return Value{}, true, false
		}
		values := append(slicesClone(owned.Values), inherited...)
		return uniqueStructureValueSequence(values, e.options.Structure), true, true
	}
	owned, handled, ok := e.structureOwnedFeatures(element)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Type", "feature", Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Type::ownedFeature",
		})
		return Value{}, true, false
	}
	if element.Symbol == nil {
		inherited, handled, ok := e.structureInheritedFeatures(element)
		if !handled || !ok {
			e.recordUnsupportedReason(element, "Type", "feature", Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: "Type::inheritedFeature",
			})
			return Value{}, true, false
		}
		values := append(slicesClone(owned.Values), inherited.Values...)
		return uniqueStructureValueSequence(values, e.options.Structure), true, true
	}
	if !e.completeMembers(element.Symbol) {
		e.recordMemberCompletenessFailure(element, "Type", "feature", element.Symbol)
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		e.recordUnsupportedReason(element, "Type", "feature", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Type::feature has unavailable semantic membership sources",
		})
		return Value{}, true, false
	}
	inherited, ok := e.structureInheritedSymbols(element, all)
	if !ok {
		e.recordUnsupportedReason(element, "Type", "feature", Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::feature owned specialization endpoints are unavailable",
		})
		return Value{}, true, false
	}
	values := append(slicesClone(owned.Values), symbolSequence(inherited).Values...)
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
}

func (e *Evaluator) structureDirectedFeatures(element Element, property string) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, "Type", property, reason)
		return Value{}, true, false
	}
	if property == "input" {
		send, known := e.isMetaclass(element, "SendActionUsage")
		if known && send {
			members, handled, ok := e.structureOwnedMembers(element)
			if !handled || !ok {
				return unsupported(Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "SendActionUsage::input owned arguments are unavailable",
				})
			}
			var inputs []Value
			for _, member := range members.Values {
				if member.Kind != ElementValue {
					return unsupported(Reason{
						Category: ReasonUnrepresentedEndpoint,
						Detail:   "SendActionUsage::input owned argument is not an element",
					})
				}
				expression, known := e.isMetaclass(member.Element, "Expression")
				if !known {
					return unsupported(Reason{
						Category: ReasonUnrepresentedEndpoint,
						Detail:   "SendActionUsage::input owned argument has no represented metaclass",
					})
				}
				if expression {
					inputs = append(inputs, member)
				}
			}
			return sequence(inputs...), true, true
		}
	}
	ownedFeatures, ownedHandled, ownedOK := e.structureOwnedFeatures(element)
	if !ownedHandled || !ownedOK {
		return unsupported(Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::" + property + " owned feature memberships are unavailable",
		})
	}
	owned := make(map[ElementKey]bool, len(ownedFeatures.Values))
	for _, feature := range ownedFeatures.Values {
		if feature.Kind != ElementValue {
			return unsupported(Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Type::" + property + " owned feature is not an element",
			})
		}
		owned[feature.Element.Key()] = true
	}
	features, handled, ok := e.structureEffectiveFeatures(element)
	if !handled || !ok {
		return unsupported(Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Type::feature",
		})
	}
	var directed []Value
	for _, feature := range features.Values {
		if feature.Kind != ElementValue {
			return unsupported(Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Type::" + property + " effective feature is not an element",
			})
		}
		direction, known := e.structureEffectiveDirection(element, feature.Element)
		if !known {
			reason := Reason{
				Category: ReasonIncompleteStructure,
				Detail:   "Type::" + property + " effective direction is unavailable",
			}
			if featureSymbol := e.structureElementSymbol(feature.Element); featureSymbol != nil {
				reason.Detail += " for " + symbols.FQNOf(featureSymbol)
				if e.resolver != nil && e.resolver.Index() != nil && e.resolver.Index().Library(featureSymbol) {
					if _, ok := identity.LibraryCatalog(e.resolver.Index()).ElementForSymbol(featureSymbol); ok {
						reason.Category = ReasonMissingStructure
						reason.Detail += " (catalogued library feature has no direction in the minimal stub)"
					} else {
						reason.Category = ReasonLibraryTargetWithoutCatalogIdentity
						reason.Detail = symbols.FQNOf(featureSymbol)
					}
				}
			} else if id := e.structureIdentityLabel(feature.Element); id != "" {
				reason.Detail += " for " + id
			} else if feature.Element.Aspect != "" {
				reason.Detail += " for " + feature.Element.Aspect
			} else {
				reason.Detail += " because its semantic symbol is unavailable"
			}
			return unsupported(reason)
		}
		if property == "directedFeature" {
			if direction == ast.DirNone {
				continue
			}
		} else {
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

func (e *Evaluator) structureEffectiveDirection(owner, feature Element) (ast.FeatureDirection, bool) {
	if isConjugatedPortDefinition, known := e.isMetaclass(owner, "ConjugatedPortDefinition"); known &&
		isConjugatedPortDefinition {
		original, ok := e.Property(owner, "ConjugatedPortDefinition", "originalPortDefinition")
		if !ok || original.Kind != ElementValue {
			return ast.DirNone, false
		}
		direction, ok := e.structureEffectiveDirection(original.Element, feature)
		if !ok {
			return ast.DirNone, false
		}
		return semantics.ConjugateDirection(direction), true
	}
	if isPortUsage, known := e.isMetaclass(owner, "PortUsage"); known && isPortUsage {
		relationships, known := e.options.Structure.OwnedRelationships(owner)
		if !known {
			return ast.DirNone, false
		}
		for _, relationship := range relationships {
			isConjugatedTyping, known := e.isMetaclass(relationship, "ConjugatedPortTyping")
			if !known {
				return ast.DirNone, false
			}
			if !isConjugatedTyping {
				continue
			}
			conjugated, known := e.relationshipEndpoints(relationship, "conjugatedPortDefinition")
			if !known || len(conjugated) != 1 || conjugated[0].Kind != ElementValue {
				return ast.DirNone, false
			}
			original, ok := e.Property(
				conjugated[0].Element, "ConjugatedPortDefinition", "originalPortDefinition",
			)
			if !ok || original.Kind != ElementValue {
				return ast.DirNone, false
			}
			direction, ok := e.structureEffectiveDirection(original.Element, feature)
			if !ok {
				return ast.DirNone, false
			}
			return semantics.ConjugateDirection(direction), true
		}
	}

	featureSymbol := e.structureElementSymbol(feature)
	ownerSymbol := e.structureElementSymbol(owner)
	if ownerSymbol != nil {
		if featureSymbol != nil {
			portUsage, portUsageKnown := e.isMetaclass(owner, "PortUsage")
			portDefinition, portDefinitionKnown := e.isMetaclass(owner, "PortDefinition")
			if e.model != nil && (portUsageKnown && portUsage || portDefinitionKnown && portDefinition) {
				for _, portFeature := range e.model.PortFeatures(ownerSymbol) {
					if portFeature.Symbol == featureSymbol {
						return portFeature.Direction, true
					}
				}
			}
			direction := e.model.EffectiveDirection(ownerSymbol, featureSymbol)
			if direction != ast.DirNone {
				return direction, true
			}
		}
	}
	value, ok := e.options.Structure.Attribute(feature, "direction")
	var direction string
	if ok {
		switch value.Kind {
		case EnumValue:
			direction = value.Enum
		case StringValue:
			direction = value.String
		case NullValue:
		default:
			return ast.DirNone, false
		}
	}
	switch direction {
	case ast.DirIn.String():
		return ast.DirIn, true
	case ast.DirOut.String():
		return ast.DirOut, true
	case ast.DirInOut.String():
		return ast.DirInOut, true
	case ast.DirNone.String():
		return ast.DirNone, true
	}

	if featureSymbol != nil && e.model != nil {
		if ownerSymbol == nil {
			if declaringOwner := featureSymbol.Owner(); declaringOwner != nil {
				return e.model.EffectiveDirection(declaringOwner, featureSymbol), true
			}
		}
		if direction := e.model.DeclaredDirection(featureSymbol); direction != ast.DirNone {
			return direction, true
		}
		switch ElementOf(featureSymbol).Membership.kind {
		case "ReturnParameterMembership":
			return ast.DirOut, true
		case "ParameterMembership":
			return ast.DirIn, true
		}
	}

	membership, handled, known := e.structureOwningMembership(feature)
	if !handled || !known {
		return ast.DirNone, false
	}
	var membershipElement Element
	switch membership.Kind {
	case MembershipValue:
		membershipElement = MembershipElement(membership.Membership)
	case ElementValue:
		membershipElement = membership.Element
	case NullValue:
		return ast.DirNone, true
	default:
		return ast.DirNone, false
	}
	isReturn, known := e.isMetaclass(membershipElement, "ReturnParameterMembership")
	if !known {
		return ast.DirNone, false
	}
	if isReturn {
		return ast.DirOut, true
	}
	isParameter, known := e.isMetaclass(membershipElement, "ParameterMembership")
	if !known {
		return ast.DirNone, false
	}
	if isParameter {
		return ast.DirIn, true
	}
	return ast.DirNone, true
}

func (e *Evaluator) structureElementSymbol(element Element) *symbols.Symbol {
	if element.Symbol != nil {
		return element.Symbol
	}
	if element.Node == nil || e.model == nil {
		return nil
	}
	var scope *symbols.Scope
	if element.Container != nil {
		scope = element.Container.Scope
	}
	symbol, found := e.model.SymbolDeclaring(scope, element.Node)
	if !found {
		return nil
	}
	return symbol
}

func (e *Evaluator) structureDirectedUsages(element Element) (Value, bool, bool) {
	directed, handled, ok := e.structureDirectedFeatures(element, "directedFeature")
	if !handled || !ok {
		definingClass := "Usage"
		if isDefinition, known := e.isMetaclass(element, "Definition"); known && isDefinition {
			definingClass = "Definition"
		}
		e.recordUnsupportedReason(
			element, definingClass, "directedUsage",
			Reason{Category: ReasonDependsOnUnsupported, Dependency: "Type::directedFeature"},
		)
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
	if element.Symbol == nil {
		values, ok := e.structureInheritedValues(element, "feature")
		if !ok {
			e.recordUnsupportedReason(element, "Type", "inheritedFeature", Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "Type::inheritedFeature has unresolved specialization references",
			})
			return Value{}, true, false
		}
		return sequence(values...), true, true
	}
	if !e.completeMembers(element.Symbol) {
		e.recordMemberCompletenessFailure(element, "Type", "inheritedFeature", element.Symbol)
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		e.recordUnsupportedReason(element, "Type", "inheritedFeature", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Type::inheritedFeature has unavailable semantic membership sources",
		})
		return Value{}, true, false
	}
	inherited, ok := e.structureInheritedSymbols(element, all)
	if !ok {
		e.recordUnsupportedReason(element, "Type", "inheritedFeature", Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::inheritedFeature owned specialization endpoints are unavailable",
		})
		return Value{}, true, false
	}
	return symbolSequence(inherited), true, true
}

func (e *Evaluator) structureEffectiveFeatureMemberships(element Element) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFilter(element, "FeatureMembership", true)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Type", "featureMembership", Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::featureMembership has an unrepresented owned FeatureMembership endpoint",
		})
		return Value{}, true, false
	}
	if element.Symbol == nil {
		inherited, ok := e.structureInheritedValues(element, "featureMembership")
		if !ok {
			e.recordUnsupportedReason(element, "Type", "featureMembership", Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "Type::featureMembership has unresolved specialization references",
			})
			return Value{}, true, false
		}
		values := append(slicesClone(owned.Values), inherited...)
		return uniqueStructureValueSequence(values, e.options.Structure), true, true
	}
	if !e.completeMembers(element.Symbol) {
		e.recordMemberCompletenessFailure(element, "Type", "featureMembership", element.Symbol)
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "feature")
	if !ok {
		e.recordUnsupportedReason(element, "Type", "featureMembership", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Type::featureMembership has unavailable semantic membership sources",
		})
		return Value{}, true, false
	}
	inheritedSymbols, ok := e.structureInheritedSymbols(element, all)
	if !ok {
		e.recordUnsupportedReason(element, "Type", "featureMembership", Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::featureMembership owned specialization endpoints are unavailable",
		})
		return Value{}, true, false
	}
	inherited, ok := e.structureFeatureMemberships(inheritedSymbols)
	if !ok {
		e.recordUnsupportedReason(element, "Type", "featureMembership", Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Type::inheritedFeature",
			Detail:     "inherited features could not be classified as FeatureMemberships",
		})
		return Value{}, true, false
	}
	values := append(slicesClone(owned.Values), membershipSequence(inherited).Values...)
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
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
	if element.Symbol == nil {
		values, ok := e.structureInheritedValues(element, "membership")
		if !ok {
			e.recordUnsupportedReason(element, "Type", "inheritedMembership", Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "Type::inheritedMembership has unresolved specialization references",
			})
			return Value{}, true, false
		}
		return sequence(values...), true, true
	}
	if !e.completeMembers(element.Symbol) {
		e.recordMemberCompletenessFailure(element, "Type", "inheritedMembership", element.Symbol)
		return Value{}, true, false
	}
	all, ok := e.model.ReflectiveElements(element.Symbol, "member")
	if !ok {
		e.recordUnsupportedReason(element, "Type", "inheritedMembership", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Type::inheritedMembership has unavailable semantic membership sources",
		})
		return Value{}, true, false
	}
	inherited, ok := e.structureInheritedSymbols(element, all)
	if !ok {
		e.recordUnsupportedReason(element, "Type", "inheritedMembership", Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   "Type::inheritedMembership owned specialization endpoints are unavailable",
		})
		return Value{}, true, false
	}
	return membershipSequence(memberships(inherited)), true, true
}

func (e *Evaluator) structureInheritedSymbols(element Element, members []*symbols.Symbol) ([]*symbols.Symbol, bool) {
	owner := e.structureElementSymbol(element)
	if owner == nil {
		return nil, false
	}
	supertypes := make(map[*symbols.Symbol]bool)
	visited := make(map[ElementKey]bool)
	var visit func(Element) bool
	visit = func(current Element) bool {
		if visited[current.Key()] {
			return true
		}
		visited[current.Key()] = true
		bases, ok := e.structureOwnedSupertypes(current)
		if !ok {
			return false
		}
		for _, base := range bases {
			symbol := e.structureElementSymbol(base)
			if symbol == nil {
				return false
			}
			supertypes[symbol] = true
			if !visit(base) {
				return false
			}
		}
		return true
	}
	if !visit(element) {
		return nil, false
	}
	inherited := make([]*symbols.Symbol, 0, len(members))
	for _, member := range members {
		if member == nil || member.Owner() == owner || !supertypes[member.Owner()] {
			continue
		}
		if e.resolver != nil && !e.resolver.InheritedMemberVisible(owner, member) {
			continue
		}
		inherited = append(inherited, member)
	}
	return inherited, true
}

func (e *Evaluator) structureOwnedSupertypes(element Element) ([]Element, bool) {
	conjugators, handled, ok := e.structureOwnedFilter(element, "Conjugation", false)
	if !handled || !ok {
		return nil, false
	}
	if len(conjugators.Values) > 1 {
		return nil, false
	}
	if len(conjugators.Values) == 1 {
		if conjugators.Values[0].Kind != ElementValue {
			return nil, false
		}
		originals, ok := e.relationshipEndpoints(conjugators.Values[0].Element, "originalType")
		if !ok {
			return nil, false
		}
		bases := make([]Element, 0, len(originals))
		for _, original := range originals {
			if original.Kind != ElementValue {
				return nil, false
			}
			isType, known := e.isMetaclass(original.Element, "Type")
			if !known {
				return nil, false
			}
			if isType {
				bases = append(bases, original.Element)
			}
		}
		return uniqueStructureElements(bases, e.options.Structure), true
	}
	return e.structureGeneralTypes(element)
}

func (e *Evaluator) structureInheritedValues(element Element, property string) ([]Value, bool) {
	if e.options.Structure == nil || e.model == nil {
		return nil, false
	}
	bases, ok := e.structureOwnedSupertypes(element)
	if !ok {
		return nil, false
	}
	var values []Value
	visited := make(map[ElementKey]bool)
	for _, base := range bases {
		inherited, ok := e.structureTypeMembers(base, property, visited)
		if !ok {
			return nil, false
		}
		values = append(values, inherited...)
	}
	return uniqueStructureValueSequence(values, e.options.Structure).Values, true
}

func (e *Evaluator) structureGeneralTypes(element Element) ([]Element, bool) {
	specializations, handled, ok := e.structureOwnedFilter(element, "Specialization", false)
	if !handled || !ok {
		return nil, false
	}
	var bases []Element
	for _, specialization := range specializations.Values {
		if specialization.Kind != ElementValue {
			return nil, false
		}
		general, ok := e.relationshipEndpoints(specialization.Element, "general")
		if !ok {
			return nil, false
		}
		for _, value := range general {
			if value.Kind != ElementValue {
				return nil, false
			}
			isType, known := e.isMetaclass(value.Element, "Type")
			if !known {
				return nil, false
			}
			if isType {
				bases = append(bases, value.Element)
			}
		}
	}
	kind := source.KindSysML
	if documentKind, known := e.elementDocumentKind(element); known {
		kind = documentKind
	}
	var impliedBases []string
	if metaclass, known := e.options.Structure.Metaclass(element); known {
		impliedBases = e.model.ImpliedBaseFQNsForMetaclass(metaclass, kind)
	}
	if len(impliedBases) == 0 && element.Node != nil {
		impliedBases = e.model.ImpliedBaseFQNs(element.Node, kind)
	}
	for _, fqn := range impliedBases {
		symbols := e.resolver.Index().LookupQualified(fqn)
		if len(symbols) == 0 {
			return nil, false
		}
		base := ElementOf(symbols[0])
		isType, known := e.isMetaclass(base, "Type")
		if !known {
			return nil, false
		}
		if isType {
			bases = append(bases, base)
		}
	}
	return uniqueStructureElements(bases, e.options.Structure), true
}

func (e *Evaluator) elementDocumentKind(element Element) (source.Kind, bool) {
	symbol := element.Symbol
	if symbol == nil {
		symbol = element.Container
	}
	if symbol == nil {
		symbol = element.Membership.Owner
	}
	if symbol == nil || e.resolver == nil {
		return 0, false
	}
	kind := e.resolver.Index().DocumentKind(symbol.DocName)
	return kind, kind != 0
}

func (e *Evaluator) structureTypeMembers(element Element, property string, visited map[ElementKey]bool) ([]Value, bool) {
	if visited[element.Key()] {
		return nil, true
	}
	visited[element.Key()] = true
	owned, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return nil, false
	}
	var public, protected []Value
	for _, membershipValue := range owned.Values {
		if membershipValue.Kind != MembershipValue {
			return nil, false
		}
		membership := Element{Membership: membershipValue.Membership, IsMembership: true}
		visibility, known := e.structureMembershipVisibility(membership)
		if !known {
			return nil, false
		}
		if !visibility.public && !visibility.protected {
			continue
		}
		if visibility.public {
			public = append(public, membershipValue)
		} else {
			protected = append(protected, membershipValue)
		}
	}
	inherited := make([]Value, 0)
	bases, ok := e.structureOwnedSupertypes(element)
	if !ok {
		return nil, false
	}
	for _, base := range bases {
		members, ok := e.structureNonPrivateMemberships(base, visited)
		if !ok {
			return nil, false
		}
		inherited = append(inherited, members...)
	}
	inherited, ok = e.structureRemoveRedefinedMemberships(element, inherited)
	if !ok {
		return nil, false
	}
	memberships := uniqueStructureValueSequence(
		append(append(public, protected...), inherited...), e.options.Structure,
	).Values
	if property == "membership" {
		return memberships, true
	}
	var values []Value
	for _, membershipValue := range memberships {
		if membershipValue.Kind != MembershipValue {
			return nil, false
		}
		membership := Element{Membership: membershipValue.Membership, IsMembership: true}
		isFeature, known := e.isMetaclass(membership, "FeatureMembership")
		if !known {
			return nil, false
		}
		if !isFeature {
			continue
		}
		if property == "featureMembership" {
			values = append(values, membershipValue)
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			return nil, false
		}
		for _, member := range members {
			if property == "feature" {
				isFeature, known := e.isMetaclass(member, "Feature")
				if !known {
					return nil, false
				}
				if !isFeature {
					continue
				}
				values = append(values, elementValue(member))
			} else {
				values = append(values, elementValue(member))
			}
		}
	}
	return uniqueStructureValueSequence(values, e.options.Structure).Values, true
}

func (e *Evaluator) structureNonPrivateMemberships(
	element Element,
	visited map[ElementKey]bool,
) ([]Value, bool) {
	if visited[element.Key()] {
		return nil, true
	}
	visited[element.Key()] = true
	owned, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return nil, false
	}
	var public, protected []Value
	for _, membershipValue := range owned.Values {
		if membershipValue.Kind != MembershipValue {
			return nil, false
		}
		membership := Element{Membership: membershipValue.Membership, IsMembership: true}
		visibility, known := e.structureMembershipVisibility(membership)
		if !known {
			return nil, false
		}
		if visibility.public {
			public = append(public, membershipValue)
		} else if visibility.protected {
			protected = append(protected, membershipValue)
		}
	}
	inherited := make([]Value, 0)
	bases, ok := e.structureOwnedSupertypes(element)
	if !ok {
		return nil, false
	}
	for _, base := range bases {
		members, ok := e.structureNonPrivateMemberships(base, visited)
		if !ok {
			return nil, false
		}
		inherited = append(inherited, members...)
	}
	inherited, ok = e.structureRemoveRedefinedMemberships(element, inherited)
	if !ok {
		return nil, false
	}
	return uniqueStructureValueSequence(
		append(append(public, protected...), inherited...), e.options.Structure,
	).Values, true
}

func (e *Evaluator) structureMembershipVisibility(
	membership Element,
) (struct {
	public    bool
	protected bool
}, bool) {
	value, ok := e.options.Structure.Attribute(membership, "visibility")
	if !ok {
		return struct {
			public    bool
			protected bool
		}{}, false
	}
	switch value.Kind {
	case EnumValue:
		return struct {
			public    bool
			protected bool
		}{public: value.Enum == "public", protected: value.Enum == "protected"}, true
	case StringValue:
		return struct {
			public    bool
			protected bool
		}{public: value.String == "public", protected: value.String == "protected"}, true
	default:
		return struct {
			public    bool
			protected bool
		}{}, false
	}
}

func (e *Evaluator) structureRemoveRedefinedMemberships(
	owner Element,
	memberships []Value,
) ([]Value, bool) {
	type candidate struct {
		value     Value
		feature   Element
		isFeature bool
		redefined map[ElementKey]bool
	}
	candidates := make([]candidate, 0, len(memberships))
	for _, membershipValue := range memberships {
		if membershipValue.Kind != MembershipValue {
			return nil, false
		}
		membership := Element{Membership: membershipValue.Membership, IsMembership: true}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			return nil, false
		}
		entry := candidate{value: membershipValue}
		for _, member := range members {
			isFeature, known := e.isMetaclass(member, "Feature")
			if !known {
				return nil, false
			}
			if !isFeature {
				continue
			}
			if len(members) != 1 {
				return nil, false
			}
			entry.feature = member
			entry.isFeature = true
			redefined, ok := e.structureAllRedefinedFeatures(member)
			if !ok {
				return nil, false
			}
			entry.redefined = make(map[ElementKey]bool, len(redefined))
			for _, feature := range redefined {
				entry.redefined[feature.Key()] = true
			}
		}
		candidates = append(candidates, entry)
	}

	ownedFeatures, handled, ok := e.structureOwnedFeatures(owner)
	if !handled || !ok {
		return nil, false
	}
	ownedRedefined := make(map[ElementKey]bool)
	for _, value := range ownedFeatures.Values {
		if value.Kind != ElementValue {
			return nil, false
		}
		redefinitions, handled, ok := e.structureOwnedFilter(value.Element, "Redefinition", false)
		if !handled || !ok {
			return nil, false
		}
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
				ownedRedefined[target.Element.Key()] = true
			}
		}
	}

	var result []Value
	for i, entry := range candidates {
		if entry.isFeature {
			removed := false
			for j, other := range candidates {
				if i != j && other.isFeature && other.redefined[entry.feature.Key()] {
					removed = true
					break
				}
			}
			if !removed {
				for redefined := range ownedRedefined {
					if entry.redefined[redefined] {
						removed = true
						break
					}
				}
			}
			if removed {
				continue
			}
		}
		result = append(result, entry.value)
	}
	return result, true
}

func (e *Evaluator) structureAllRedefinedFeatures(feature Element) ([]Element, bool) {
	seen := map[ElementKey]bool{feature.Key(): true}
	queue := []Element{feature}
	result := []Element{feature}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		redefinitions, handled, ok := e.structureOwnedFilter(current, "Redefinition", false)
		if !handled || !ok {
			return nil, false
		}
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
				if seen[target.Element.Key()] {
					continue
				}
				seen[target.Element.Key()] = true
				result = append(result, target.Element)
				queue = append(queue, target.Element)
			}
		}
	}
	return result, true
}

func uniqueStructureElements(elements []Element, structure Structure) []Element {
	values := uniqueStructureValueSequence(structureSequence(elements).Values, structure).Values
	unique := make([]Element, 0, len(values))
	for _, value := range values {
		if value.Kind == ElementValue {
			unique = append(unique, value.Element)
		}
	}
	return unique
}

func (e *Evaluator) structureVariantProperty(element Element, class, property string) (Value, bool, bool) {
	owned, handled, ok := e.structureOwnedFilter(element, "VariantMembership", true)
	if !handled || !ok {
		e.recordUnsupportedReason(
			element, class, property,
			Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   class + "::" + property + " owned Membership elements are unavailable",
			},
		)
		return Value{}, true, false
	}
	var variants []Value
	for _, value := range owned.Values {
		if value.Kind != MembershipValue {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + property + " effective member is not a Membership",
				},
			)
			return Value{}, true, false
		}
		membership := Element{Membership: value.Membership, IsMembership: true}
		isVariant, known := e.isMetaclass(membership, "VariantMembership")
		if !known {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + property + " effective membership has no represented metaclass",
				},
			)
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
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   class + "::" + property + " VariantMembership memberElement is unavailable",
				},
			)
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterValues(structureSequence(members).Values, "Usage")
		if !ok {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   class + "::" + property + " VariantMembership memberElement is not a represented Usage",
				},
			)
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
	for _, feature := range features.Values {
		if feature.Kind != ElementValue {
			return Value{}, true, false
		}
		isEnd, ok := e.Property(feature.Element, "Feature", "isEnd")
		if !ok || isEnd.Kind != BooleanValue {
			return Value{}, true, false
		}
		if isEnd.Boolean {
			ends = append(ends, feature)
		}
	}
	return sequence(ends...), true, true
}

func (e *Evaluator) structureEffectiveEndFeatures(
	element Element,
	class, property string,
) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, class, property, reason)
		return Value{}, true, false
	}
	owned, handled, ok := e.structureOwnedEndFeatures(element)
	if !handled || !ok {
		return unsupported(
			Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Type::endFeature owned end-feature memberships are unresolved"},
		)
	}
	if element.Node != nil {
		isSuccession, known := e.isMetaclass(element, "SuccessionAsUsage")
		if !known {
			return unsupported(Reason{Category: ReasonMissingStructure, Detail: "Type::endFeature succession metaclass is unavailable"})
		}
		if isSuccession {
			if len(owned.Values) != 2 {
				return unsupported(
					Reason{Category: ReasonIncompleteStructure, Detail: "Type::endFeature succession requires two positional owned ends"},
				)
			}
			return sequence(owned.Values...), true, true
		}
		if _, found, known := e.structureActionSuccessionFeatures(element); !known {
			return unsupported(
				Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Type::endFeature positional transition ends are unresolved"},
			)
		} else if found {
			if len(owned.Values) != 2 {
				return unsupported(
					Reason{Category: ReasonIncompleteStructure, Detail: "Type::endFeature transition requires two positional owned ends"},
				)
			}
			return sequence(owned.Values...), true, true
		}
	}
	if element.Symbol != nil && e.model != nil {
		features := e.model.EndFeatures(element.Symbol)
		if len(features) == 0 {
			return sequence(owned.Values...), true, true
		}
		values := make([]Value, 0, len(features))
		hasFlowEnds := false
		if usage, ok := element.Symbol.Decl.(*ast.Usage); ok && usage.FlowEnds != nil {
			hasFlowEnds = usage.FlowEnds.From != nil || usage.FlowEnds.To != nil
		}
		for index, feature := range features {
			if index < len(owned.Values) {
				values = append(values, owned.Values[index])
				continue
			}
			if feature != nil {
				if hasFlowEnds && e.resolver != nil && e.resolver.Index() != nil &&
					e.resolver.Index().Library(feature) {
					if _, represented := e.options.Structure.Metaclass(ElementOf(feature)); !represented {
						if _, identified := identity.LibraryCatalog(e.resolver.Index()).ElementForSymbol(feature); identified {
							return unsupported(Reason{
								Category: ReasonMissingStructure,
								Detail:   "Type::endFeature implied flow end is absent from Structure",
							})
						}
						return unsupported(Reason{
							Category: ReasonLibraryTargetWithoutCatalogIdentity,
							Detail:   symbols.FQNOf(feature),
						})
					}
				}
				values = append(values, elementValue(ElementOf(feature)))
				continue
			}
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "Type::endFeature contains an unresolved end"})
		}
		return sequence(values...), true, true
	}
	inherited, handled, ok := e.structureInheritedFeatures(element)
	if !handled || !ok {
		return unsupported(Reason{Category: ReasonDependsOnUnsupported, Dependency: "Type::inheritedFeature"})
	}
	var inheritedEnds []Value
	for _, feature := range inherited.Values {
		if feature.Kind != ElementValue {
			return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Type::endFeature inherited feature is not an element"})
		}
		isEnd, known := e.Property(feature.Element, "Feature", "isEnd")
		if !known || isEnd.Kind != BooleanValue {
			return unsupported(
				Reason{Category: ReasonMissingStructure, Detail: "Type::endFeature inherited Feature::isEnd attribute is unavailable"},
			)
		}
		if isEnd.Boolean {
			inheritedEnds = append(inheritedEnds, feature)
		}
	}
	values := append([]Value(nil), owned.Values...)
	for index, feature := range inheritedEnds {
		if index >= len(owned.Values) {
			values = append(values, feature)
		}
	}
	return sequence(values...), true, true
}

func (e *Evaluator) structureFeatureTypes(element Element) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, "Feature", "type", reason)
		return Value{}, true, false
	}
	value, handled, ok := e.structureFeatureTypesFrom(element, make(map[ElementKey]bool))
	if !handled || !ok {
		return unsupported(
			Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::type a FeatureTyping, Subsetting, or FeatureChaining target is unavailable in Structure"},
		)
	}
	if len(value.Values) != 0 {
		return value, handled, ok
	}
	relationships, known := e.options.Structure.OwnedRelationships(element)
	if !known {
		return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::type owned typing and subsetting relationships are unavailable"})
	}
	for _, relationship := range relationships {
		isCrossSubsetting, known := e.isMetaclass(relationship, "CrossSubsetting")
		if !known {
			return unsupported(Reason{Category: ReasonUnrepresentedElement, Detail: "Feature::type owned relationship metaclass is unavailable"})
		}
		if isCrossSubsetting {
			continue
		}
		for _, metaclass := range []string{"FeatureTyping", "Subsetting"} {
			matches, known := e.isMetaclass(relationship, metaclass)
			if !known {
				return unsupported(Reason{Category: ReasonUnrepresentedElement, Detail: "Feature::type owned relationship metaclass is unavailable"})
			}
			if matches {
				return unsupported(Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "Feature::type owned " + metaclass + " target is unavailable",
				})
			}
		}
	}
	chaining, handled, ok := e.structureFeatureChaining(element)
	if !handled || !ok {
		return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::type FeatureChaining endpoints are unavailable"})
	}
	if len(chaining.Values) != 0 {
		return unsupported(Reason{Category: ReasonDependsOnUnsupported, Dependency: "Feature::chainingFeature"})
	}
	return value, true, true
}

func (e *Evaluator) structureFeatureTypesFrom(element Element, visiting map[ElementKey]bool) (Value, bool, bool) {
	key := structureElementKey(element, e.options.Structure)
	if visiting[key] {
		return sequence(), true, true
	}
	visiting[key] = true
	defer delete(visiting, key)

	conjugators, conjugatorsHandled, conjugatorsOK := e.structureOwnedFilter(element, "Conjugation", false)
	if conjugatorsHandled && conjugatorsOK && len(conjugators.Values) != 0 {
		if len(conjugators.Values) != 1 || conjugators.Values[0].Kind != ElementValue {
			return Value{}, true, false
		}
		originalTypes, ok := e.relationshipEndpoints(conjugators.Values[0].Element, "originalType")
		if !ok {
			return Value{}, true, false
		}
		originalFeatures, ok := e.structureFilterValues(originalTypes, "Feature")
		if !ok {
			return Value{}, true, false
		}
		var values []Value
		for _, original := range originalFeatures {
			if original.Kind != ElementValue {
				return Value{}, true, false
			}
			types, handled, ok := e.structureFeatureTypesFrom(original.Element, visiting)
			if !handled || !ok {
				return Value{}, true, false
			}
			values = append(values, types.Values...)
		}
		return e.mostSpecificFeatureTypes(values)
	}
	if conjugatorsHandled && !conjugatorsOK && element.Symbol == nil {
		return Value{}, true, false
	}

	relationships, ok := e.options.Structure.OwnedRelationships(element)
	if !ok {
		if element.Symbol != nil {
			types, supported := e.featureTypes(element.Symbol)
			if supported {
				return symbolSequence(types), true, true
			}
		}
		return Value{}, true, false
	}
	values := make([]Value, 0)
	hasConjugatedPortTyping := false
	for _, relationship := range relationships {
		conjugatedPortTyping, known := e.isMetaclass(relationship, "ConjugatedPortTyping")
		if !known {
			return Value{}, true, false
		}
		if conjugatedPortTyping {
			hasConjugatedPortTyping = true
			targets, known := e.relationshipEndpoints(relationship, "conjugatedPortDefinition")
			if !known {
				return Value{}, true, false
			}
			filtered, known := e.structureFilterValues(targets, "Type")
			if !known || len(filtered) == 0 {
				return Value{}, true, false
			}
			values = append(values, filtered...)
			continue
		}
		matches, known := e.isMetaclass(relationship, "FeatureTyping")
		if !known {
			return Value{}, true, false
		}
		if !matches {
			continue
		}
		var targets []Value
		var targetsKnown bool
		if e.hasStatedRelatedElement(relationship, "type") {
			stated, known := e.statedRelatedElements(relationship, "type")
			if !known {
				return Value{}, true, false
			}
			targets, targetsKnown = e.structureFilterElements(stated, "Type")
			if !targetsKnown {
				return Value{}, true, false
			}
		} else {
			targets, targetsKnown = e.relationshipEndpoints(relationship, "type")
			if !targetsKnown {
				return Value{}, true, false
			}
			targets, targetsKnown = e.structureFilterValues(targets, "Type")
			if !targetsKnown {
				return Value{}, true, false
			}
		}
		if !targetsKnown || len(targets) == 0 {
			return Value{}, true, false
		}
		values = append(values, targets...)
	}

	for _, relationship := range relationships {
		subsetting, known := e.isMetaclass(relationship, "Subsetting")
		if !known {
			return Value{}, true, false
		}
		if !subsetting {
			continue
		}
		crossSubsetting, known := e.isMetaclass(relationship, "CrossSubsetting")
		if !known {
			return Value{}, true, false
		}
		if crossSubsetting {
			continue
		}
		metaclass, known := e.options.Structure.Metaclass(relationship)
		if !known {
			return Value{}, true, false
		}
		targetProperty := "subsettedFeature"
		switch {
		case e.options.Structure.Specializes(metaclass, "Redefinition"):
			targetProperty = "redefinedFeature"
		case e.options.Structure.Specializes(metaclass, "ReferenceSubsetting"):
			targetProperty = "referencedFeature"
		}
		targets, ok := e.relationshipEndpoints(relationship, targetProperty)
		if !ok {
			return Value{}, true, false
		}
		targets, ok = e.structureFilterValues(targets, "Feature")
		if !ok || len(targets) == 0 {
			return Value{}, true, false
		}
		for _, target := range targets {
			if target.Kind != ElementValue {
				return Value{}, true, false
			}
			types, handled, ok := e.structureFeatureTypesFrom(target.Element, visiting)
			if !handled || !ok {
				return Value{}, true, false
			}
			values = append(values, types.Values...)
		}
	}

	chaining, handled, ok := e.structureFeatureChaining(element)
	if !handled || !ok {
		return Value{}, true, false
	}

	typeSources := make([]*symbols.Symbol, 0, 2)
	if element.Symbol != nil && element.Symbol.IsFeature() {
		typeSources = append(typeSources, element.Symbol)
	}
	if instantiated, ok := e.instantiationExpressionProperty(element, "instantiatedType"); ok &&
		instantiated.Kind == ElementValue && instantiated.Element.Symbol != nil &&
		instantiated.Element.Symbol.IsFeature() {
		typeSource := instantiated.Element.Symbol
		if len(typeSources) == 0 || typeSources[0] != typeSource {
			typeSources = append(typeSources, typeSource)
		}
	}
	if len(typeSources) > 0 {
		for _, typeSource := range typeSources {
			if hasConjugatedPortTyping {
				continue
			}
			if !e.completeFeatureTypeInputs(typeSource) {
				return Value{}, true, false
			}
			inherited := e.model.FeatureTypeSet(typeSource)
			if len(inherited) == 0 {
				if _, hasBase := e.model.FeatureBaseFQN(typeSource); hasBase &&
					len(values) == 0 && len(chaining.Values) == 0 {
					return Value{}, true, false
				}
				continue
			}
			for _, target := range inherited {
				if target == nil {
					return Value{}, true, false
				}
				value := elementValue(ElementOf(target))
				alreadyOwned := false
				for _, owned := range values {
					if owned.Kind == ElementValue &&
						sameStructureElement(owned.Element, value.Element, e.options.Structure) {
						alreadyOwned = true
						break
					}
				}
				if !alreadyOwned {
					values = append(values, value)
				}
			}
		}
	}

	if len(chaining.Values) > 0 {
		last := chaining.Values[len(chaining.Values)-1]
		if last.Kind != ElementValue {
			return Value{}, true, false
		}
		types, handled, ok := e.structureFeatureTypesFrom(last.Element, visiting)
		if !handled || !ok {
			return Value{}, true, false
		}
		values = append(values, types.Values...)
	}
	return e.mostSpecificFeatureTypes(values)
}

func (e *Evaluator) mostSpecificFeatureTypes(values []Value) (Value, bool, bool) {
	values = uniqueStructureValueSequence(values, e.options.Structure).Values
	if len(values) < 2 {
		return sequence(values...), true, true
	}
	types := make([]Value, 0, len(values))
	for i, candidate := range values {
		if candidate.Kind != ElementValue {
			return Value{}, true, false
		}
		redundant := false
		for j, other := range values {
			if i == j {
				continue
			}
			if other.Kind != ElementValue {
				return Value{}, true, false
			}
			specializes, known := e.featureTypeConforms(other.Element, candidate.Element)
			if !known {
				return Value{}, true, false
			}
			if specializes {
				redundant = true
				break
			}
		}
		if !redundant {
			types = append(types, candidate)
		}
	}
	return sequence(types...), true, true
}

func (e *Evaluator) featureTypeConforms(specific, general Element) (bool, bool) {
	if sameStructureElement(specific, general, e.options.Structure) {
		return true, true
	}
	if specific.Symbol == nil || general.Symbol == nil || e.model == nil ||
		!e.completeSupertypes(specific.Symbol) || !e.completeSupertypes(general.Symbol) {
		return false, false
	}
	return e.model.Conforms(specific.Symbol, general.Symbol), true
}

func (e *Evaluator) structureTypedDefinition(
	element Element,
	expected string,
	many bool,
	class, property string,
) (Value, bool, bool) {
	types, handled, ok := e.structureFeatureTypes(element)
	if !handled || !ok {
		e.recordUnsupportedReason(
			element, class, property,
			Reason{Category: ReasonDependsOnUnsupported, Dependency: "Feature::type"},
		)
		return Value{}, true, false
	}
	var definitions []Value
	for _, typ := range types.Values {
		matches, known := e.isMetaclass(typ.Element, expected)
		if !known {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + property + " type target has no represented metaclass",
				},
			)
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
		e.recordUnsupportedReason(
			element, class, property,
			Reason{
				Category: ReasonIncompleteStructure,
				Detail:   class + "::" + property + " multiple type targets violate single-valued cardinality",
			},
		)
		return Value{}, true, false
	}
	if len(definitions) == 0 {
		return nullValue(), true, true
	}
	return definitions[0], true, true
}

func (e *Evaluator) structureIndividualDefinition(element Element) (Value, bool, bool) {
	definitions, handled, ok := e.structureTypedDefinition(
		element, "OccurrenceDefinition", true, "OccurrenceUsage", "individualDefinition",
	)
	if !handled || !ok {
		return Value{}, true, false
	}
	var individuals []Value
	for _, definition := range definitions.Values {
		if definition.Kind != ElementValue {
			e.recordUnsupportedReason(
				element, "OccurrenceUsage", "individualDefinition",
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "OccurrenceUsage::individualDefinition typed definition is not represented as an Element",
				},
			)
			return Value{}, true, false
		}
		isIndividual, known := e.options.Structure.Attribute(definition.Element, "isIndividual")
		if !known || isIndividual.Kind != BooleanValue {
			e.recordUnsupportedReason(
				element, "OccurrenceUsage", "individualDefinition",
				Reason{
					Category: ReasonMissingStructure,
					Detail:   "OccurrenceUsage::individualDefinition target OccurrenceDefinition has no isIndividual attribute",
				},
			)
			return Value{}, true, false
		}
		if isIndividual.Boolean {
			individuals = append(individuals, definition)
		}
	}
	if len(individuals) == 0 {
		return nullValue(), true, true
	}
	return individuals[0], true, true
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
	isEnd, ok := e.Property(element, "Feature", "isEnd")
	if !ok {
		return Value{}, true, false
	}
	if isEnd.Kind != BooleanValue || !isEnd.Boolean {
		return nullValue(), true, true
	}
	return e.structureOwningType(element)
}

func (e *Evaluator) structureFeatureIsEnd(element Element) (Value, bool, bool) {
	if structure, ok := e.options.Structure.(StatedAttributeStructure); ok {
		if value, stated := structure.StatedAttribute(element, "isEnd"); stated &&
			value.Kind == BooleanValue {
			return value, true, true
		}
	}
	isFlowEnd, known := e.isMetaclass(element, "FlowEnd")
	if !known {
		return Value{}, true, false
	}
	if isFlowEnd {
		return booleanValue(true), true, true
	}
	if element.Symbol != nil && e.model != nil {
		value, ok := e.model.ReflectiveFeatureValue(element.Symbol, "isEnd")
		if !ok || value.Kind != symbols.FilterValueBool {
			return Value{}, true, false
		}
		return booleanValue(value.Bool), true, true
	}
	if e.options.Structure != nil {
		if value, ok := e.options.Structure.Attribute(element, "isEnd"); ok &&
			value.Kind == BooleanValue {
			return value, true, true
		}
	}
	return booleanValue(false), true, true
}

func (e *Evaluator) structureUnmaterializedEndName(element Element, property string) (Value, bool, bool) {
	isEnd, ok := e.Property(element, "Feature", "isEnd")
	if !ok || isEnd.Kind != BooleanValue || !isEnd.Boolean {
		return Value{}, false, false
	}
	owningType, handled, ok := e.structureOwningType(element)
	if !handled || !ok || owningType.Kind != ElementValue {
		return Value{}, handled, ok
	}
	isFlowEnd, known := e.isMetaclass(element, "FlowEnd")
	if !known {
		return Value{}, true, false
	}
	if isFlowEnd {
		isFlowUsage, known := e.isMetaclass(owningType.Element, "FlowUsage")
		if !known {
			return Value{}, true, false
		}
		if isFlowUsage {
			redefinitions, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
			if !handled || !ok {
				return Value{}, true, false
			}
			if len(redefinitions.Values) == 0 {
				if name, ok := e.structureFlowEndName(owningType.Element, element, property); ok {
					return name, true, true
				}
				e.recordUnsupportedReason(
					element,
					"Element",
					property,
					Reason{
						Category: ReasonImpliedRelationshipNotMaterialized,
						Detail:   "FlowEnd::" + property + " requires its implied Redefinition",
					},
				)
				return Value{}, true, false
			}
			return Value{}, false, false
		}
	}
	isSuccession, known := e.isMetaclass(owningType.Element, "SuccessionAsUsage")
	if !known {
		return Value{}, true, false
	}
	if !isSuccession {
		return Value{}, false, false
	}
	redefinitions, handled, ok := e.structureOwnedFilter(element, "Redefinition", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	if len(redefinitions.Values) > 0 {
		return Value{}, false, false
	}
	return nullValue(), true, true
}

func (e *Evaluator) structureFlowEndName(flow, flowEnd Element, property string) (Value, bool) {
	flowEnds, handled, ok := e.structureFlowProperty(flow, "flowEnd")
	if !handled || !ok {
		return Value{}, false
	}
	position := -1
	for index, value := range flowEnds.Values {
		if value.Kind == ElementValue &&
			sameStructureElement(value.Element, flowEnd, e.options.Structure) {
			position = index
			break
		}
	}
	if position < 0 {
		return Value{}, false
	}

	types, ok := e.structureFlowEndTypes(flow)
	if !ok || len(types) == 0 {
		return Value{}, false
	}
	var result Value
	for _, typ := range types {
		name, ok := e.structureTypedEndName(typ, position, property)
		if !ok {
			return Value{}, false
		}
		if result.Kind == 0 {
			result = name
			continue
		}
		if result.Kind != name.Kind || result.String != name.String {
			return Value{}, false
		}
	}
	return result, result.Kind != 0
}

func (e *Evaluator) structureFlowEndTypes(flow Element) ([]Element, bool) {
	typings, handled, ok := e.structureOwnedFilter(flow, "FeatureTyping", false)
	if !handled || !ok {
		return nil, false
	}
	if len(typings.Values) > 0 {
		var types []Element
		hasStatedType := false
		for _, typing := range typings.Values {
			if typing.Kind != ElementValue {
				return nil, false
			}
			targets, ok := e.relationshipEndpoints(typing.Element, "type")
			if !ok && e.hasUnresolvedRelatedElement(typing.Element, "type") {
				return nil, false
			}
			if len(targets) == 0 {
				if e.hasUnresolvedRelatedElement(typing.Element, "type") {
					return nil, false
				}
				continue
			}
			filtered, ok := e.structureFilterValues(targets, "Type")
			if !ok || len(filtered) == 0 {
				return nil, false
			}
			hasStatedType = true
			for _, target := range filtered {
				if target.Kind != ElementValue {
					return nil, false
				}
				types = append(types, target.Element)
			}
		}
		if hasStatedType {
			return uniqueStructureElements(types, e.options.Structure), true
		}
	}
	if e.model == nil || e.resolver == nil || e.resolver.Index() == nil {
		return nil, false
	}
	metaclass, known := e.options.Structure.Metaclass(flow)
	if !known {
		return nil, false
	}
	kind := source.KindSysML
	if documentKind, known := e.elementDocumentKind(flow); known {
		kind = documentKind
	}
	bases := e.model.ImpliedBaseFQNsForMetaclass(metaclass, kind)
	var types []Element
	for _, fqn := range bases {
		for _, base := range e.resolver.Index().LookupQualified(fqn) {
			if base == nil || !base.IsFeature() {
				continue
			}
			for _, typ := range e.model.FeatureTypes(base) {
				if typ != nil {
					types = append(types, ElementOf(typ))
				}
			}
			break
		}
	}
	return uniqueStructureElements(types, e.options.Structure), len(types) > 0
}

func (e *Evaluator) structureTypedEndName(typ Element, position int, property string) (Value, bool) {
	if symbol := e.structureFlowTypeSymbol(typ); symbol != nil && e.model != nil {
		ends := e.model.EndFeatures(symbol)
		if position >= len(ends) || ends[position] == nil {
			return Value{}, false
		}
		end := ends[position]
		name := e.model.EffectiveNameOf(end)
		if property == "shortName" {
			name = e.model.EffectiveShortNameOf(end)
		}
		if name == "" {
			return nullValue(), true
		}
		return stringValue(name), true
	}

	ends, handled, ok := e.structureProperty(typ, "Type", "endFeature")
	if !handled || !ok || ends.Kind != SequenceValue || position >= len(ends.Values) {
		return Value{}, false
	}
	end := ends.Values[position]
	if end.Kind != ElementValue {
		return Value{}, false
	}
	name, handled, ok := e.structureName(end.Element, property)
	return name, handled && ok
}

func (e *Evaluator) structureFlowTypeSymbol(element Element) *symbols.Symbol {
	if element.Symbol != nil {
		return element.Symbol
	}
	if e.options.Structure == nil || e.resolver == nil || e.resolver.Index() == nil {
		return nil
	}
	structure, ok := e.options.Structure.(ElementIdentityStructure)
	if !ok {
		return nil
	}
	id, ok := structure.ElementIdentity(element)
	if !ok {
		return nil
	}
	library, ok := identity.LibraryCatalog(e.resolver.Index()).Element(id)
	if !ok {
		return nil
	}
	return library.Symbol
}

func (e *Evaluator) structureFeatureTarget(element Element) (Value, bool, bool) {
	chains, handled, ok := e.structureFeatureChaining(element)
	if !handled || !ok {
		e.recordUnsupportedReason(
			element, "Feature", "featureTarget",
			Reason{Category: ReasonDependsOnUnsupported, Dependency: "Feature::chainingFeature"},
		)
		return Value{}, true, false
	}
	if len(chains.Values) == 0 {
		return elementValue(element), true, true
	}
	last := chains.Values[len(chains.Values)-1]
	if last.Kind != ElementValue {
		e.recordUnsupportedReason(
			element, "Feature", "featureTarget",
			Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::featureTarget final FeatureChaining target is not represented as an Element"},
		)
		return Value{}, true, false
	}
	return last, true, true
}

func (e *Evaluator) structureFeaturingTypes(
	element Element,
	seen map[ElementKey]bool,
	class, property string,
) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, class, property, reason)
		return Value{}, true, false
	}
	key := element.Key()
	if seen[key] {
		return unsupported(Reason{
			Category: ReasonIncompleteStructure,
			Detail:   class + "::" + property + " has a recursive featuring type",
		})
	}
	seen[key] = true
	relations, handled, ok := e.structureOwnedFilter(element, "TypeFeaturing", false)
	if !handled || !ok {
		return unsupported(Reason{
			Category: ReasonUnrepresentedEndpoint,
			Detail:   class + "::" + property + " owned TypeFeaturing relationships are unavailable",
		})
	}
	values := make([]Value, 0)
	for _, relation := range relations.Values {
		types, ok := e.relationshipEndpoints(relation.Element, "featuringType")
		if !ok {
			return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "TypeFeaturing::featuringType"})
		}
		types, ok = e.structureFilterValues(types, "Type")
		if !ok {
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "TypeFeaturing::featuringType has an unresolved range"})
		}
		values = append(values, types...)
	}
	owningMembership, handled, ok := e.structureOwningFeatureMembership(element)
	if !handled || !ok {
		return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::owningFeatureMembership"})
	}
	if owningMembership.Kind == MembershipValue {
		owningType, handled, ok := e.structureOwningType(element)
		if !handled || !ok {
			return unsupported(Reason{Category: ReasonDependsOnUnsupported, Dependency: "Feature::owningType"})
		}
		if owningType.Kind == ElementValue {
			values = append(values, owningType)
		}
	}
	chains, handled, ok := e.structureFeatureChaining(element)
	if !handled || !ok {
		return unsupported(Reason{Category: ReasonDependsOnUnsupported, Dependency: "Feature::chainingFeature"})
	}
	if len(chains.Values) > 0 {
		if chains.Values[0].Kind != ElementValue {
			return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Feature::chainingFeature"})
		}
		features, handled, ok := e.structureFeaturingTypes(
			chains.Values[0].Element, seen, class, property,
		)
		if !handled || !ok {
			return unsupported(Reason{
				Category:   ReasonDependsOnUnsupported,
				Dependency: class + "::" + property,
			})
		}
		values = append(values, features.Values...)
	}
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
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
	if nameProperty == "name" || nameProperty == "shortName" {
		return e.structureName(members[0], nameProperty)
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
		if handled {
			e.recordUnsupportedReason(element, "Namespace", "membership", Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Namespace::membership has an unrepresented owned Membership endpoint",
			})
		}
		return Value{}, handled, false
	}
	if element.Symbol == nil {
		e.recordUnsupportedReason(element, "Namespace", "membership", Reason{
			Category: ReasonUnrepresentedElement,
			Detail:   "Namespace::membership has no semantic namespace symbol",
		})
		return Value{}, true, false
	}
	if !e.completeMembers(element.Symbol) {
		e.recordMemberCompletenessFailure(element, "Namespace", "membership", element.Symbol)
		return Value{}, true, false
	}
	members, ok := e.model.ReflectiveElements(element.Symbol, "member")
	if !ok {
		e.recordUnsupportedReason(element, "Namespace", "membership", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Namespace::membership has unavailable semantic membership sources",
		})
		return Value{}, true, false
	}
	inherited := memberships(e.inheritedSymbols(element.Symbol, members))
	imported, handled, ok := e.structureImportedMemberships(element)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Namespace", "membership", Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "Namespace::membership contains an unresolved imported reference",
		})
		return Value{}, true, false
	}
	values := append(append(slicesClone(owned.Values), membershipSequence(inherited).Values...), imported.Values...)
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
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
		e.recordUnsupportedReason(element, "Namespace", "member", Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Namespace::membership",
		})
		return Value{}, true, false
	}
	var values []Value
	for _, membership := range memberships.Values {
		handle := Element{Membership: membership.Membership, IsMembership: true}
		members, ok := e.structureMembershipMembers(handle)
		if !ok {
			e.recordUnsupportedReason(element, "Namespace", "member", Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   "Namespace::member has an unrepresented membership memberElement endpoint",
			})
			return Value{}, true, false
		}
		for _, member := range members {
			values = append(values, elementValue(member))
		}
	}
	return uniqueStructureValueSequence(values, e.options.Structure), true, true
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

func (e *Evaluator) structureFirstParameter(element Element) (Value, bool, bool) {
	parameters, handled, ok := e.structureStepParameters(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if len(parameters.Values) == 0 {
		return nullValue(), true, true
	}
	return parameters.Values[0], true, true
}

func (e *Evaluator) structurePerformedAction(element Element) (Value, bool, bool) {
	target, handled, ok := e.structureReferencedFeature(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	if target.Symbol == nil && target.Node == nil && !target.IsMembership && target.Aspect == "" {
		return elementValue(element), true, true
	}
	targetValue, ok := e.Property(target, "Feature", "featureTarget")
	if !ok {
		return Value{}, true, false
	}
	return targetValue, true, true
}

func (e *Evaluator) structureReferencedFeature(element Element) (Element, bool, bool) {
	references, handled, ok := e.structureOwnedFilter(element, "ReferenceSubsetting", false)
	if !handled || !ok {
		return Element{}, true, false
	}
	if len(references.Values) == 0 {
		return Element{}, true, true
	}
	if len(references.Values) != 1 || references.Values[0].Kind != ElementValue {
		return Element{}, true, false
	}
	targets, ok := e.relationshipEndpoints(references.Values[0].Element, "referencedFeature")
	if !ok || len(targets) != 1 || targets[0].Kind != ElementValue {
		return Element{}, true, false
	}
	return targets[0].Element, true, true
}

func (e *Evaluator) structureActionArgument(element Element, property string, oneBasedIndex int) (Value, bool, bool) {
	parameters, handled, ok := e.structureOwnedFilter(element, "ParameterMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var arguments []Element
	for _, parameter := range parameters.Values {
		if parameter.Kind != MembershipValue {
			return Value{}, true, false
		}
		members, ok := e.structureMembershipMembers(MembershipElement(parameter.Membership))
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
				arguments = append(arguments, member)
			}
		}
	}
	if oneBasedIndex > len(arguments) {
		return nullValue(), true, true
	}
	valuations, handled, ok := e.structureOwnedFilter(arguments[oneBasedIndex-1], "FeatureValue", false)
	if !handled || !ok {
		return Value{}, true, false
	}
	if len(valuations.Values) == 0 {
		return nullValue(), true, true
	}
	var valuation Element
	switch valuations.Values[0].Kind {
	case ElementValue:
		valuation = valuations.Values[0].Element
	case MembershipValue:
		valuation = MembershipElement(valuations.Values[0].Membership)
	default:
		return Value{}, true, false
	}
	values, ok := e.relationshipEndpoints(valuation, "value")
	if !ok {
		return Value{}, true, false
	}
	expressions, ok := e.structureFilterValues(values, "Expression")
	if !ok {
		return Value{}, true, false
	}
	return structurePropertyCardinality(expressions, true)
}

func (e *Evaluator) structureBehaviorParameters(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureEffectiveFeatureMemberships(element)
	if !handled || !ok {
		e.recordUnsupportedReason(
			element, "Behavior", "parameter",
			Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Behavior::parameter effective FeatureMembership endpoints are unavailable"},
		)
		return Value{}, true, false
	}
	var parameters []Value
	for _, value := range memberships.Values {
		membership := Element{Membership: value.Membership, IsMembership: true}
		isParameter, known := e.isMetaclass(membership, "ParameterMembership")
		if !known {
			e.recordUnsupportedReason(
				element, "Behavior", "parameter",
				Reason{Category: ReasonUnrepresentedElement, Detail: "Behavior::parameter effective membership has no represented metaclass"},
			)
			return Value{}, true, false
		}
		if !isParameter {
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			e.recordUnsupportedReason(
				element, "Behavior", "parameter",
				Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Behavior::parameter ParameterMembership memberElement is unavailable"},
			)
			return Value{}, true, false
		}
		filtered, handled, ok := e.structureFilterRelated(members, "Feature", false)
		if !handled || !ok {
			e.recordUnsupportedReason(
				element, "Behavior", "parameter",
				Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Behavior::parameter ParameterMembership member is not a represented Feature"},
			)
			return Value{}, true, false
		}
		parameters = append(parameters, filtered.Values...)
	}
	return sequence(parameters...), true, true
}

func (e *Evaluator) structureStepParameters(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "ParameterMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var parameters []Value
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			return Value{}, true, false
		}
		members, ok := e.structureMembershipMembers(Element{
			Membership:   value.Membership,
			IsMembership: true,
		})
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
		isConnector, known := e.isMetaclass(element, "Connector")
		if !known {
			return Value{}, true, false
		}
		if isConnector {
			related, ok := e.Property(element, "Connector", "relatedFeature")
			return related, true, ok
		}
		var endpoints []Value
		for _, name := range []string{"source", "target"} {
			value, ok := e.Property(element, "Relationship", name)
			if !ok {
				return Value{}, true, false
			}
			switch value.Kind {
			case NullValue:
			case ElementValue:
				endpoints = append(endpoints, value)
			case SequenceValue:
				endpoints = append(endpoints, value.Values...)
			default:
				return Value{}, true, false
			}
		}
		return sequence(endpoints...), true, true
	case "owningType":
		return e.structureRelationshipOwner(element, "Type")
	case "conjugatedPortDefinition":
		return e.structureRelationshipOwner(element, "ConjugatedPortDefinition")
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
		value, handled, ok := e.structureRelationshipOwner(element, "")
		if !handled || !ok {
			e.recordUnsupportedReason(
				element, class, name,
				Reason{Category: ReasonUnrepresentedEndpoint, Detail: "Documentation::documentedElement has no represented owning related element"},
			)
		}
		return value, handled, ok
	case "owningClassifier":
		return e.structureRelationshipOwner(element, "Classifier")
	case "owningFeatureOfType":
		return e.structureRelationshipOwner(element, "Feature")
	case "source", "target", "special", "general", "specific", "typedFeature", "type",
		"subsettingFeature", "subsettedFeature", "redefiningFeature", "redefinedFeature",
		"client", "supplier", "importedElement",
		"membershipOwningNamespace", "memberElement", "ownedMemberElement",
		"referencingFeature",
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

func (e *Evaluator) structureFeatureReferenceReferent(element Element) (Value, bool, bool) {
	class, property := "FeatureReferenceExpression", "referent"
	if metaclass, known := e.options.Structure.Metaclass(element); known && metaclass == "FeatureChainExpression" {
		class, property = metaclass, "targetFeature"
	}
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, class, property, reason)
		return Value{}, true, false
	}
	if e.hasUnresolvedRelatedElement(element, property) {
		return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "compact reference endpoint is unresolved"})
	}
	if e.hasStatedRelatedElement(element, property) {
		targets, ok := e.statedRelatedElements(element, property)
		if !ok || len(targets) == 0 {
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "stated compact reference endpoint is unresolved"})
		}
		filtered, ok := e.structureFilterElements(targets, "Feature")
		if !ok {
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "stated compact reference endpoint has an unresolved range"})
		}
		return structurePropertyCardinality(filtered, true)
	}
	memberships, handled, ok := e.structureOwnedFilter(element, "Membership", true)
	if !handled || !ok {
		return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "owned membership elements are unavailable"})
	}
	if len(memberships.Values) == 0 {
		if _, reference := element.Node.(*ast.FeatureReference); reference {
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: "feature reference has no resolved owned membership"})
		}
		return nullValue(), true, true
	}
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			return unsupported(Reason{Category: ReasonUnrepresentedElement, Detail: "owned membership is not a Membership"})
		}
		membership := MembershipElement(value.Membership)
		isParameter, known := e.isMetaclass(membership, "ParameterMembership")
		if !known {
			return unsupported(Reason{Category: ReasonUnrepresentedElement, Detail: "owned membership metaclass is unavailable"})
		}
		if isParameter {
			continue
		}
		members, ok := e.structureMembershipMembers(membership)
		if !ok {
			detail := "owned membership member is unresolved"
			if id := e.structureIdentityLabel(membership); id != "" {
				detail += ": " + id
			}
			return unsupported(Reason{Category: ReasonUnresolvedReference, Detail: detail})
		}
		if len(members) == 0 {
			return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "owned membership has no member"})
		}
		for _, member := range members {
			if sameStructureElement(member, element, e.options.Structure) {
				return unsupported(Reason{Category: ReasonNotApplicable, Detail: "owned membership resolves to the expression itself"})
			}
		}
		isFeature, known := e.isMetaclass(members[0], "Feature")
		if !known {
			return unsupported(Reason{Category: ReasonUnrepresentedElement, Detail: "owned membership member metaclass is unavailable"})
		}
		if !isFeature {
			return unsupported(Reason{Category: ReasonUnrepresentedEndpoint, Detail: "first non-parameter owned member is not a Feature"})
		}
		return elementValue(members[0]), true, true
	}
	return nullValue(), true, true
}

func (e *Evaluator) structureConnectorRelatedFeatures(element Element) (Value, bool, bool) {
	unsupported := func(reason Reason) (Value, bool, bool) {
		e.recordUnsupportedReason(element, "Connector", "relatedFeature", reason)
		return Value{}, true, false
	}
	ends, handled, ok := e.structureOwnedEndFeatures(element)
	if !handled || !ok {
		return unsupported(Reason{
			Category: ReasonMissingStructure,
			Detail:   "Connector::relatedFeature effective end features are unavailable",
		})
	}
	if len(ends.Values) == 0 {
		return unsupported(Reason{
			Category: ReasonMissingStructure,
			Detail:   "Connector::relatedFeature has no owned end features",
		})
	}
	var features []Value
	for _, end := range ends.Values {
		if end.Kind != ElementValue {
			return unsupported(Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Connector::relatedFeature end feature is not represented as an Element",
			})
		}
		references, handled, ok := e.structureOwnedFilter(end.Element, "ReferenceSubsetting", false)
		if !handled || !ok {
			return unsupported(Reason{
				Category: ReasonMissingStructure,
				Detail:   "Connector::relatedFeature end ReferenceSubsetting relationships are unavailable",
			})
		}
		for _, reference := range references.Values {
			if reference.Kind != ElementValue {
				return unsupported(Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Connector::relatedFeature ReferenceSubsetting is not represented as an Element",
				})
			}
			targets, ok := e.relationshipEndpoints(reference.Element, "referencedFeature")
			if !ok {
				if e.hasUnresolvedRelatedElement(reference.Element, "referencedFeature") {
					return unsupported(Reason{
						Category: ReasonUnresolvedReference,
						Detail:   "Connector::relatedFeature ReferenceSubsetting target is unresolved",
					})
				}
				return unsupported(Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   "Connector::relatedFeature ReferenceSubsetting target is unavailable",
				})
			}
			filtered, ok := e.structureFilterValues(targets, "Feature")
			if !ok {
				return unsupported(Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Connector::relatedFeature target metaclass is unavailable",
				})
			}
			features = append(features, filtered...)
		}
	}
	return sequence(features...), true, true
}

func (e *Evaluator) structureConnectorEndpoint(element Element, endpoint string) (Value, bool, bool) {
	property := "targetFeature"
	if endpoint == "source" {
		property = "sourceFeature"
	}
	features, handled, ok := e.structureConnectorRelatedFeatures(element)
	if !handled || !ok {
		e.recordUnsupportedReason(element, "Connector", property, Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "connector related features are unresolved",
		})
		return Value{}, true, false
	}
	if endpoint == "source" {
		if len(features.Values) == 0 {
			return nullValue(), true, true
		}
		return features.Values[0], true, true
	}
	if len(features.Values) < 2 {
		return sequence(), true, true
	}
	return sequence(features.Values[1:]...), true, true
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
	if e.hasUnresolvedRelatedElement(element, property) {
		return nil, false
	}
	if (actual == "FeatureValue" || e.options.Structure.Specializes(actual, "FeatureValue")) && property == "value" {
		members, ok := e.structureMembershipMembers(element)
		if !ok || len(members) != 1 {
			return nil, false
		}
		return []Value{elementValue(members[0])}, true
	}
	isConnector := e.options.Structure.Specializes(actual, "Connector")
	if isConnector && oneOf(property, "source", "target", "relatedElement", "sourceFeature", "targetFeature", "relatedFeature") {
		if property == "target" || property == "targetFeature" {
			value, handled, ok := e.structureConnectorEndpoint(element, "target")
			return value.Values, handled && ok
		}
		if property == "source" || property == "sourceFeature" {
			value, handled, ok := e.structureConnectorEndpoint(element, "source")
			return value.Values, handled && ok
		}
		value, handled, ok := e.structureConnectorRelatedFeatures(element)
		return value.Values, handled && ok
	}
	if property == "relatedElement" {
		source, sourceOK := e.relationshipEndpoints(element, "source")
		target, targetOK := e.relationshipEndpoints(element, "target")
		if !sourceOK || !targetOK {
			return nil, false
		}
		return sequence(append(source, target...)...).Values, true
	}
	if related, ok := e.options.Structure.RelatedElements(element, property); ok && len(related) > 0 {
		values := make([]Value, 0, len(related))
		for _, target := range related {
			values = append(values, elementValue(target))
		}
		return values, true
	}
	if e.hasStatedRelatedElement(element, property) {
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
			setEndpoint("owning")
		} else if property == "target" || property == "superclassifier" {
			setEndpoint("owned")
		}
	case specializes("FeatureTyping"):
		if property == "source" || property == "typedFeature" {
			setEndpoint("owning")
		} else if property == "target" || property == "type" {
			setEndpoint("owned")
		}
	case specializes("ReferenceSubsetting"):
		if property == "source" || property == "referencingFeature" {
			setEndpoint("owning")
		} else if property == "target" || property == "referencedFeature" || property == "subsettedFeature" {
			setEndpoint("owned")
		}
	case specializes("Redefinition"):
		if property == "source" || property == "redefiningFeature" {
			setEndpoint("owning")
		} else if property == "target" || property == "redefinedFeature" {
			setEndpoint("owned")
		}
	case specializes("TypeFeaturing"):
		if property == "source" || property == "featureOfType" {
			setEndpoint("owning")
		} else if property == "target" || property == "featuringType" {
			setEndpoint("owned")
		}
	case specializes("FeatureChaining"):
		if property == "source" || property == "featureChained" {
			setEndpoint("owning")
		} else if property == "target" || property == "chainingFeature" {
			setEndpoint("chainingFeature")
		}
	case specializes("CrossSubsetting"):
		if property == "source" || property == "crossingFeature" {
			setEndpoint("owning")
		} else if property == "target" || property == "crossedFeature" {
			setEndpoint("owned")
		}
	case specializes("FeatureInverting"):
		if property == "source" || property == "featureInverted" {
			setEndpoint("owning")
		} else if property == "target" || property == "invertingFeature" {
			setEndpoint("owned")
		}
	case specializes("Subsetting"):
		if property == "source" || property == "subsettingFeature" {
			setEndpoint("owning")
		} else if property == "target" || property == "subsettedFeature" {
			setEndpoint("owned")
		}
	case specializes("Specialization"):
		if property == "source" || property == "specific" || property == "special" {
			setEndpoint("owning")
		} else if property == "target" || property == "general" {
			setEndpoint("owned")
		} else if property == "referencingFeature" {
			setEndpoint("owning")
		}
	case specializes("Membership"):
		if property == "source" || property == "membershipOwningNamespace" {
			setEndpoint("owning")
		} else if property == "target" || property == "memberElement" {
			setEndpoint("owned")
		}
	case specializes("Import"):
		if property == "source" || property == "importOwningNamespace" {
			setEndpoint("owning")
		} else if property == "target" || property == "importedElement" {
			setEndpoint("importedElement")
		}
	case specializes("FeatureValue"):
		if property == "source" || property == "featureWithValue" ||
			property == "membershipOwningNamespace" {
			setEndpoint("owning")
		} else if property == "target" || property == "value" {
			setEndpoint("owned")
		}
	case specializes("Annotation"):
		if property == "source" || property == "annotatingElement" ||
			property == "owningAnnotatingElement" || property == "ownedAnnotatingElement" {
			setEndpoint("owning")
		} else if property == "target" || property == "annotatedElement" {
			setEndpoint("owned")
		}
	case specializes("Dependency"):
		if property == "source" || property == "client" {
			setEndpoint("owning")
		} else if property == "target" || property == "supplier" {
			setEndpoint("owned")
		}
	case specializes("Conjugation"):
		if property == "source" || property == "conjugatedType" {
			setEndpoint("owning")
		} else if property == "target" || property == "originalType" {
			setEndpoint("owned")
		}
	case specializes("Disjoining"):
		if property == "source" || property == "typeDisjoined" {
			setEndpoint("owning")
		} else if property == "target" || property == "disjoiningType" {
			setEndpoint("owned")
		}
	case specializes("Differencing"):
		if property == "source" || property == "typeDifferenced" {
			setEndpoint("owning")
		} else if property == "target" || property == "differencingType" {
			setEndpoint("owned")
		}
	case specializes("Intersecting"):
		if property == "source" || property == "typeIntersected" {
			setEndpoint("owning")
		} else if property == "target" || property == "intersectingType" {
			setEndpoint("owned")
		}
	case specializes("Unioning"):
		if property == "source" || property == "typeUnioned" {
			setEndpoint("owning")
		} else if property == "target" || property == "unioningType" {
			setEndpoint("owned")
		}
	case specializes("TypeFeaturing"):
		if property == "source" || property == "featureOfType" || property == "owningFeatureOfType" {
			setEndpoint("owning")
		} else if property == "target" || property == "featuringType" {
			setEndpoint("owned")
		}
	case specializes("Connector"):
		if property == "source" || property == "sourceFeature" {
			setEndpoint("sourceFeature")
		} else if property == "target" || property == "targetFeature" {
			setEndpoint("targetFeature")
		}
	default:
		if property == "source" {
			setEndpoint("owning")
		} else if property == "target" {
			setEndpoint("owned")
		}
	}
	if endpoint == "" && property == "specific" && specializes("Specialization") {
		setEndpoint("owning")
	}
	if endpoint == "" && property == "general" && specializes("Specialization") {
		setEndpoint("owned")
	}
	if endpoint == "" {
		return nil, false
	}
	if endpoint == "chainingFeature" {
		related, known := e.options.Structure.RelatedElements(element, "chainingFeature")
		if !known {
			return nil, false
		}
		values := make([]Value, 0, len(related))
		for _, target := range related {
			values = append(values, elementValue(target))
		}
		return values, true
	}
	if endpoint == "owning" {
		alias := property
		if property == "source" || property == "specific" {
			alias = e.relationshipEndpointAlias(actual, endpoint)
		}
		owner, present, known := e.options.Structure.OwningRelatedElement(element)
		if !known {
			related, known := e.options.Structure.RelatedElements(element, alias)
			if !known {
				return nil, false
			}
			values := make([]Value, 0, len(related))
			for _, target := range related {
				values = append(values, elementValue(target))
			}
			return values, true
		}
		if present {
			return []Value{elementValue(owner)}, true
		}
		related, relatedKnown := e.options.Structure.RelatedElements(element, alias)
		if relatedKnown && len(related) > 0 {
			values := make([]Value, 0, len(related))
			for _, target := range related {
				values = append(values, elementValue(target))
			}
			return values, true
		}
		return []Value{}, true
	}
	if endpoint == "owned" {
		alias := property
		if property == "target" || property == "general" {
			alias = e.relationshipEndpointAlias(actual, endpoint)
		}
		related, ok := e.options.Structure.OwnedRelatedElements(element)
		if endpoints, aliasKnown := e.options.Structure.RelatedElements(element, alias); aliasKnown && len(endpoints) > 0 {
			related, ok = endpoints, true
		}
		if !ok {
			return nil, false
		}
		if len(related) == 0 {
			if relationship, isRelationship := element.Node.(*ast.Relationship); isRelationship &&
				relationship != nil && relationship.Target != nil {
				return nil, false
			}
		}
		values := make([]Value, 0, len(related))
		for _, target := range related {
			values = append(values, elementValue(target))
		}
		return values, true
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
	if (!ok || len(related) == 0) &&
		oneOf(endpoint, "featureChained", "referencingFeature", "crossingFeature", "redefiningFeature", "featureOfType") {
		owner, present, known := e.options.Structure.OwningRelatedElement(element)
		if !known {
			return nil, false
		}
		if present {
			return []Value{elementValue(owner)}, true
		}
	}
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

func (e *Evaluator) relationshipEndpointAlias(actual, endpoint string) string {
	type endpointNames struct {
		class  string
		owning string
		owned  string
	}
	names := []endpointNames{
		{class: "NamespaceImport", owning: "importOwningNamespace", owned: "importedNamespace"},
		{class: "MembershipImport", owning: "importOwningNamespace", owned: "importedMembership"},
		{class: "ReferenceSubsetting", owning: "referencingFeature", owned: "referencedFeature"},
		{class: "Redefinition", owning: "redefiningFeature", owned: "redefinedFeature"},
		{class: "FeatureTyping", owning: "typedFeature", owned: "type"},
		{class: "CrossSubsetting", owning: "crossingFeature", owned: "crossedFeature"},
		{class: "FeatureInverting", owning: "featureInverted", owned: "invertingFeature"},
		{class: "FeatureChaining", owning: "featureChained", owned: "chainingFeature"},
		{class: "Subsetting", owning: "subsettingFeature", owned: "subsettedFeature"},
		{class: "Subclassification", owning: "subclassifier", owned: "superclassifier"},
		{class: "Specialization", owning: "specific", owned: "general"},
		{class: "Membership", owning: "membershipOwningNamespace", owned: "memberElement"},
		{class: "FeatureValue", owning: "featureWithValue", owned: "value"},
		{class: "Annotation", owning: "annotatingElement", owned: "annotatedElement"},
		{class: "Import", owning: "importOwningNamespace", owned: "importedElement"},
		{class: "Dependency", owning: "client", owned: "supplier"},
		{class: "Conjugation", owning: "conjugatedType", owned: "originalType"},
		{class: "Disjoining", owning: "typeDisjoined", owned: "disjoiningType"},
		{class: "Differencing", owning: "typeDifferenced", owned: "differencingType"},
		{class: "Intersecting", owning: "typeIntersected", owned: "intersectingType"},
		{class: "Unioning", owning: "typeUnioned", owned: "unioningType"},
		{class: "TypeFeaturing", owning: "featureOfType", owned: "featuringType"},
		{class: "Connector", owning: "sourceFeature", owned: "targetFeature"},
	}
	for _, candidate := range names {
		if actual != candidate.class && !e.options.Structure.Specializes(actual, candidate.class) {
			continue
		}
		if endpoint == "owning" {
			return candidate.owning
		}
		if endpoint == "owned" {
			return candidate.owned
		}
	}
	if endpoint == "owning" {
		return "source"
	}
	if endpoint == "owned" {
		return "target"
	}
	return endpoint
}

func (e *Evaluator) structureTypedRelatedValues(element Element, property, expected string, single bool) (Value, bool, bool) {
	values, ok := e.structureRelatedElements(element, property)
	if !ok {
		return Value{}, true, false
	}
	return e.structureFilterRelated(values, expected, single)
}

func (e *Evaluator) structureResultFeatures(element Element, class, property string) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "ReturnParameterMembership", true)
	if !handled || !ok {
		e.recordUnsupportedReason(
			element, class, property,
			Reason{
				Category: ReasonUnrepresentedEndpoint,
				Detail:   class + "::" + property + " return-parameter memberships are unavailable",
			},
		)
		return Value{}, true, false
	}
	var features []Value
	for _, value := range memberships.Values {
		if value.Kind != MembershipValue {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedEndpoint,
					Detail:   class + "::" + property + " return parameter is not a membership",
				},
			)
			return Value{}, true, false
		}
		members, ok := e.structureMembershipMembers(MembershipElement(value.Membership))
		if !ok {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnresolvedReference,
					Detail:   class + "::" + property + " return parameter member is unresolved",
				},
			)
			return Value{}, true, false
		}
		filtered, ok := e.structureFilterElements(members, "Feature")
		if !ok {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   class + "::" + property + " return parameter member is not a Feature",
				},
			)
			return Value{}, true, false
		}
		features = append(features, filtered...)
	}
	if len(features) == 0 && element.Symbol != nil {
		outputs, handled, ok := e.structureDirectedFeatures(element, "output")
		if !handled || !ok {
			e.recordUnsupportedReason(
				element, class, property,
				Reason{Category: ReasonDependsOnUnsupported, Dependency: "Type::output"},
			)
			return Value{}, true, false
		}
		features = outputs.Values
	}
	if len(features) == 0 {
		e.recordUnsupportedReason(
			element, class, property,
			Reason{
				Category: ReasonIncompleteStructure,
				Detail:   class + "::" + property + " has no represented return parameter or output Feature",
			},
		)
		return Value{}, true, false
	}
	return structurePropertyCardinality(features, true)
}

func (e *Evaluator) structureMultiplicityBounds(element Element, property string) (Value, bool, bool) {
	members, handled, ok := e.structureOwnedMembers(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	var expressions []Value
	for _, member := range members.Values {
		if member.Kind != ElementValue {
			continue
		}
		isExpression, known := e.isMetaclass(member.Element, "Expression")
		if !known {
			return Value{}, true, false
		}
		if isExpression {
			expressions = append(expressions, member)
		}
	}
	var lower, upper Value
	if len(expressions) >= 2 {
		lower = expressions[0]
	}
	if len(expressions) == 1 {
		upper = expressions[0]
	} else if len(expressions) >= 2 {
		upper = expressions[1]
	}
	switch property {
	case "lowerBound":
		if lower.Kind == 0 {
			return nullValue(), true, true
		}
		return lower, true, true
	case "upperBound":
		if upper.Kind == 0 {
			return nullValue(), true, true
		}
		return upper, true, true
	case "bound":
		values := make([]Value, 0, 2)
		if lower.Kind != 0 {
			values = append(values, lower)
		}
		if upper.Kind != 0 {
			values = append(values, upper)
		}
		return sequence(values...), true, true
	default:
		return Value{}, true, false
	}
}

func (e *Evaluator) structureInvocationArguments(element Element) (Value, bool, bool) {
	instantiatedType, ok := e.Property(element, "InstantiationExpression", "instantiatedType")
	if !ok || instantiatedType.Kind != ElementValue {
		return Value{}, true, false
	}
	constructor, known := e.isMetaclass(element, "ConstructorExpression")
	if !known {
		return Value{}, true, false
	}
	var inputs Value
	if constructor {
		inputs, ok = e.Property(instantiatedType.Element, "Type", "feature")
	} else {
		inputs, ok = e.Property(instantiatedType.Element, "Type", "input")
	}
	if !ok || inputs.Kind != SequenceValue {
		return Value{}, true, false
	}
	argumentOwner := element
	if constructor {
		result, handled, ok := e.structureResultFeatures(element, "Expression", "result")
		if !handled || !ok || result.Kind != ElementValue {
			return Value{}, true, false
		}
		expressionFeatures, handled, ok := e.structureOwnedFeatures(element)
		if !handled || !ok {
			return Value{}, true, false
		}
		for _, feature := range expressionFeatures.Values {
			if feature.Kind != ElementValue {
				return Value{}, true, false
			}
			if !sameStructureElement(feature.Element, result.Element, e.options.Structure) {
				e.recordUnsupportedReason(
					element, "InstantiationExpression", "argument",
					Reason{
						Category: ReasonNotApplicable,
						Detail:   "ConstructorExpression owns non-result features; KerML requires its owned features to be the result",
					},
				)
				return Value{}, true, false
			}
		}
		argumentOwner = result.Element
	}
	ownedFeatures, handled, ok := e.structureOwnedFeatures(argumentOwner)
	if !handled || !ok {
		return Value{}, true, false
	}
	actualFeatures := ownedFeatures.Values
	if !constructor {
		actualFeatures, ok = e.structureOperatorInputs(argumentOwner, actualFeatures)
		if !ok {
			return Value{}, true, false
		}
	}
	formalInputs := inputs.Values
	if constructor {
		formalInputs, ok = e.structurePublicFeatures(formalInputs)
		if !ok {
			return Value{}, true, false
		}
	}
	if len(actualFeatures) > len(formalInputs) {
		return Value{}, true, false
	}

	explicit := make([][]Element, len(actualFeatures))
	hasExplicit := false
	hasPositional := false
	for index, feature := range actualFeatures {
		if feature.Kind != ElementValue {
			return Value{}, true, false
		}
		redefinitions, handled, ok := e.structureOwnedFilter(feature.Element, "Redefinition", false)
		if !handled || !ok {
			return Value{}, true, false
		}
		for _, redefinition := range redefinitions.Values {
			if redefinition.Kind != ElementValue {
				return Value{}, true, false
			}
			targets, ok := e.relationshipEndpoints(redefinition.Element, "redefinedFeature")
			if !ok || len(targets) == 0 {
				return Value{}, true, false
			}
			for _, target := range targets {
				if target.Kind != ElementValue {
					return Value{}, true, false
				}
				explicit[index] = append(explicit[index], target.Element)
			}
		}
		if len(explicit[index]) != 0 {
			hasExplicit = true
		} else {
			hasPositional = true
		}
	}
	if hasExplicit && hasPositional {
		return Value{}, true, false
	}

	bound := make([]int, len(formalInputs))
	featureValues := make([][]Value, len(formalInputs))
	for index, feature := range actualFeatures {
		formalIndex := index
		if len(explicit[index]) != 0 {
			if len(explicit[index]) != 1 {
				return Value{}, true, false
			}
			formalIndex = -1
			for candidate, input := range formalInputs {
				if input.Kind != ElementValue {
					return Value{}, true, false
				}
				if sameStructureElement(explicit[index][0], input.Element, e.options.Structure) {
					formalIndex = candidate
					break
				}
			}
			if formalIndex < 0 {
				return Value{}, true, false
			}
		}
		if formalIndex >= len(formalInputs) {
			return Value{}, true, false
		}
		bound[formalIndex]++
		if bound[formalIndex] > 1 {
			return Value{}, true, false
		}
		valuations, handled, ok := e.structureOwnedFilter(feature.Element, "FeatureValue", false)
		if !handled || !ok {
			return Value{}, true, false
		}
		for _, valuation := range valuations.Values {
			var valuationElement Element
			switch valuation.Kind {
			case ElementValue:
				valuationElement = valuation.Element
			case MembershipValue:
				valuationElement = MembershipElement(valuation.Membership)
			default:
				return Value{}, true, false
			}
			values, ok := e.relationshipEndpoints(valuationElement, "value")
			if !ok {
				return Value{}, true, false
			}
			filtered, ok := e.structureFilterValues(values, "Expression")
			if !ok {
				return Value{}, true, false
			}
			featureValues[formalIndex] = append(featureValues[formalIndex], filtered...)
		}
	}
	var arguments []Value
	for _, values := range featureValues {
		arguments = append(arguments, values...)
	}
	return sequence(arguments...), true, true
}

func (e *Evaluator) structurePublicFeatures(features []Value) ([]Value, bool) {
	public := make([]Value, 0, len(features))
	for _, feature := range features {
		if feature.Kind != ElementValue || feature.Element.Symbol == nil {
			return nil, false
		}
		if symbols.VisibleOutside(feature.Element.Symbol.Visibility) {
			public = append(public, feature)
		}
	}
	return public, true
}

func (e *Evaluator) structureOperatorInputs(element Element, ownedFeatures []Value) ([]Value, bool) {
	inputs := make([]Value, 0, len(ownedFeatures))
	for _, feature := range ownedFeatures {
		if feature.Kind != ElementValue {
			return nil, false
		}
		direction, known := e.structureEffectiveDirection(element, feature.Element)
		if !known {
			return nil, false
		}
		if direction == ast.DirIn || direction == ast.DirInOut {
			inputs = append(inputs, feature)
		}
	}
	return inputs, true
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
	if e.hasUnresolvedRelatedElement(membership, "memberElement") ||
		e.hasUnresolvedRelatedElement(membership, "ownedRelatedElement") {
		return nil, false
	}
	values, ok := e.relationshipEndpoints(membership, "memberElement")
	if !ok || len(values) == 0 {
		related, known := e.options.Structure.OwnedRelatedElements(membership)
		if !known {
			if !ok {
				return nil, false
			}
		} else {
			values = make([]Value, 0, len(related))
			for _, member := range related {
				values = append(values, elementValue(member))
			}
		}
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
	seenElements := make(map[ElementKey]bool)
	seenSymbols := make(map[symbols.ElementKey]bool)
	seenMemberships := make(map[MembershipKey]bool)
	seenMembershipSymbols := make(map[symbols.ElementKey]bool)
	unique := make([]Value, 0, len(values))
	for _, value := range values {
		switch value.Kind {
		case ElementValue:
			key := value.Element.Key()
			symbolKey := symbols.KeyOf(value.Element.Symbol)
			if key != (ElementKey{}) && seenElements[key] ||
				symbolKey != (symbols.ElementKey{}) && seenSymbols[symbolKey] {
				continue
			}
			if key != (ElementKey{}) {
				seenElements[key] = true
			}
			if symbolKey != (symbols.ElementKey{}) {
				seenSymbols[symbolKey] = true
			}
		case MembershipValue:
			membership := value.Membership
			key := membership.Key()
			symbol := membership.Symbol
			if membership.Symbol == nil {
				symbol = membership.Member
			}
			symbolKey := symbols.KeyOf(symbol)
			if key != (MembershipKey{}) && seenMemberships[key] ||
				symbolKey != (symbols.ElementKey{}) && seenMembershipSymbols[symbolKey] {
				continue
			}
			if key != (MembershipKey{}) {
				seenMemberships[key] = true
			}
			if symbolKey != (symbols.ElementKey{}) {
				seenMembershipSymbols[symbolKey] = true
			}
		default:
			unique = append(unique, value)
			continue
		}
		unique = append(unique, value)
	}
	return sequence(unique...)
}

func uniqueStructureValueSequence(values []Value, structure Structure) Value {
	values = uniqueValueSequence(values).Values
	identity, ok := structure.(ElementIdentityStructure)
	if !ok {
		return sequence(values...)
	}
	seen := make(map[string]bool)
	unique := make([]Value, 0, len(values))
	for _, value := range values {
		var element Element
		switch value.Kind {
		case ElementValue:
			element = value.Element
		case MembershipValue:
			element = MembershipElement(value.Membership)
		default:
			unique = append(unique, value)
			continue
		}
		key, known := identity.ElementIdentity(element)
		if known && key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		unique = append(unique, value)
	}
	return sequence(unique...)
}

func structureElementKey(element Element, structure Structure) ElementKey {
	if identity, ok := structure.(ElementIdentityStructure); ok {
		if value, known := identity.ElementIdentity(element); known {
			return ElementKey{aspect: "identity:" + value, isMembership: element.IsMembership}
		}
	}
	return element.Key()
}

func sameStructureElement(left, right Element, structure Structure) bool {
	if identity, ok := structure.(ElementIdentityStructure); ok {
		leftID, leftOK := identity.ElementIdentity(left)
		rightID, rightOK := identity.ElementIdentity(right)
		if leftOK && rightOK {
			return leftID == rightID
		}
	}
	return left.Key() == right.Key()
}

func slicesClone(values []Value) []Value {
	return append([]Value(nil), values...)
}
