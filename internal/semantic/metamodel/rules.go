package metamodel

import (
	"fmt"
	"slices"
	"strings"
)

var servedRules = []Rule{
	{DefiningClass: "Element", Property: "documentation", Constraint: "deriveElementDocumentation"},
	{DefiningClass: "Element", Property: "isImpliedIncluded", Basis: "owned attribute; implied relationships are not included"},
	{DefiningClass: "Element", Property: "isLibraryElement", Constraint: "deriveElementIsLibraryElement"},
	{DefiningClass: "Element", Property: "name", Constraint: "deriveElementName"},
	{DefiningClass: "Element", Property: "ownedAnnotation", Constraint: "deriveElementOwnedAnnotation"},
	{DefiningClass: "Element", Property: "ownedElement", Constraint: "deriveElementOwnedElement"},
	{DefiningClass: "Element", Property: "ownedRelationship", Basis: "owned relationships, excluding implied relationships"},
	{DefiningClass: "Element", Property: "owner", Constraint: "deriveElementOwner"},
	{DefiningClass: "Element", Property: "owningMembership", Basis: "opposite of Element::owningRelationship"},
	{DefiningClass: "Element", Property: "owningNamespace", Constraint: "deriveElementOwningNamespace"},
	{DefiningClass: "Element", Property: "qualifiedName", Constraint: "deriveElementQualifiedName"},
	{DefiningClass: "Element", Property: "shortName", Constraint: "deriveElementShortName"},
	{DefiningClass: "Element", Property: "textualRepresentation", Constraint: "deriveElementTextualRepresentation"},
	{DefiningClass: "Feature", Property: "chainingFeature", Constraint: "deriveFeatureChainingFeature"},
	{DefiningClass: "Feature", Property: "crossFeature", Constraint: "deriveFeatureCrossFeature"},
	{DefiningClass: "Feature", Property: "direction", Basis: "owned attribute; value set by the textual notation"},
	{DefiningClass: "Feature", Property: "featureTarget", Constraint: "deriveFeatureFeatureTarget"},
	{DefiningClass: "Feature", Property: "featuringType", Constraint: "deriveFeatureFeaturingType"},
	{DefiningClass: "Feature", Property: "isComposite", Basis: "owned attribute; value set by the textual notation"},
	{DefiningClass: "Feature", Property: "ownedRedefinition", Constraint: "deriveFeatureOwnedRedefinition"},
	{DefiningClass: "Feature", Property: "ownedReferenceSubsetting", Constraint: "deriveFeatureOwnedReferenceSubsetting"},
	{DefiningClass: "Feature", Property: "ownedSubsetting", Constraint: "deriveFeatureOwnedSubsetting"},
	{DefiningClass: "Feature", Property: "ownedTyping", Constraint: "deriveFeatureOwnedTyping"},
	{DefiningClass: "Feature", Property: "owningType", Basis: "subsets owningFeatureMembership.owningType"},
	{DefiningClass: "Feature", Property: "type", Constraint: "deriveFeatureType"},
	{DefiningClass: "Membership", Property: "memberElement", Basis: "redefines Relationship::target"},
	{DefiningClass: "Membership", Property: "memberName", Basis: "name of memberElement; OwningMembership redefines this as ownedMemberName"},
	{DefiningClass: "Membership", Property: "memberShortName", Basis: "shortName of memberElement; OwningMembership redefines this as ownedMemberShortName"},
	{DefiningClass: "Membership", Property: "membershipOwningNamespace", Basis: "opposite of Namespace::ownedMembership"},
	{DefiningClass: "Namespace", Property: "importedMembership", Constraint: "deriveNamespaceImportedMembership"},
	{DefiningClass: "Namespace", Property: "member", Constraint: "deriveNamespaceMembers"},
	{DefiningClass: "Namespace", Property: "membership", Basis: "ownedMembership ∪ importedMemberships(); Type adds inheritedMemberships()"},
	{DefiningClass: "Namespace", Property: "ownedImport", Constraint: "deriveNamespaceOwnedImport"},
	{DefiningClass: "Namespace", Property: "ownedMember", Constraint: "deriveNamespaceOwnedMember"},
	{DefiningClass: "Namespace", Property: "ownedMembership", Constraint: "deriveNamespaceOwnedMembership"},
	{DefiningClass: "Relationship", Property: "ownedRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "owningRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "relatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "source", Basis: "first endpoint in the relationship's relatedElement order"},
	{DefiningClass: "Relationship", Property: "target", Basis: "second endpoint in the relationship's relatedElement order"},
	{DefiningClass: "Type", Property: "differencingType", Constraint: "deriveTypeDifferencingType"},
	{DefiningClass: "Type", Property: "directedFeature", Constraint: "deriveTypeDirectedFeature"},
	{DefiningClass: "Type", Property: "endFeature", Constraint: "deriveTypeEndFeature"},
	{DefiningClass: "Type", Property: "feature", Constraint: "deriveTypeFeature"},
	{DefiningClass: "Type", Property: "featureMembership", Constraint: "deriveTypeFeatureMembership"},
	{DefiningClass: "Type", Property: "inheritedFeature", Constraint: "deriveTypeInheritedFeature"},
	{DefiningClass: "Type", Property: "inheritedMembership", Constraint: "deriveTypeInheritedMembership"},
	{DefiningClass: "Type", Property: "input", Constraint: "deriveTypeInput"},
	{DefiningClass: "Type", Property: "intersectingType", Constraint: "deriveTypeIntersectingType"},
	{DefiningClass: "Type", Property: "isConjugated", Basis: "owned Conjugation relationship"},
	{DefiningClass: "Type", Property: "multiplicity", Constraint: "deriveTypeMultiplicity"},
	{DefiningClass: "Type", Property: "output", Constraint: "deriveTypeOutput"},
	{DefiningClass: "Type", Property: "ownedEndFeature", Constraint: "deriveTypeOwnedEndFeature"},
	{DefiningClass: "Type", Property: "ownedFeature", Constraint: "deriveTypeOwnedFeature"},
	{DefiningClass: "Type", Property: "ownedFeatureMembership", Constraint: "deriveTypeOwnedFeatureMembership"},
	{DefiningClass: "Type", Property: "ownedSpecialization", Constraint: "deriveTypeOwnedSpecialization"},
	{DefiningClass: "Type", Property: "unioningType", Constraint: "deriveTypeUnioningType"},
	{DefiningClass: "Usage", Property: "definition", Basis: "redefines Feature::type"},
	{DefiningClass: "Usage", Property: "directedUsage", Constraint: "deriveUsageDirectedUsage"},
	{DefiningClass: "Usage", Property: "isReference", Constraint: "deriveUsageIsReference"},
	{DefiningClass: "Usage", Property: "nestedAction", Constraint: "deriveUsageNestedAction"},
	{DefiningClass: "Usage", Property: "nestedAllocation", Constraint: "deriveUsageNestedAllocation"},
	{DefiningClass: "Usage", Property: "nestedAnalysisCase", Constraint: "deriveUsageNestedAnalysisCase"},
	{DefiningClass: "Usage", Property: "nestedAttribute", Constraint: "deriveUsageNestedAttribute"},
	{DefiningClass: "Usage", Property: "nestedCalculation", Constraint: "deriveUsageNestedCalculation"},
	{DefiningClass: "Usage", Property: "nestedCase", Constraint: "deriveUsageNestedCase"},
	{DefiningClass: "Usage", Property: "nestedConcern", Constraint: "deriveUsageNestedConcern"},
	{DefiningClass: "Usage", Property: "nestedConnection", Constraint: "deriveUsageNestedConnection"},
	{DefiningClass: "Usage", Property: "nestedConstraint", Constraint: "deriveUsageNestedConstraint"},
	{DefiningClass: "Usage", Property: "nestedEnumeration", Constraint: "deriveUsageNestedEnumeration"},
	{DefiningClass: "Usage", Property: "nestedFlow", Constraint: "deriveUsageNestedFlow"},
	{DefiningClass: "Usage", Property: "nestedInterface", Constraint: "deriveUsageNestedInterface"},
	{DefiningClass: "Usage", Property: "nestedItem", Constraint: "deriveUsageNestedItem"},
	{DefiningClass: "Usage", Property: "nestedMetadata", Constraint: "deriveUsageNestedMetadata"},
	{DefiningClass: "Usage", Property: "nestedOccurrence", Constraint: "deriveUsageNestedOccurrence"},
	{DefiningClass: "Usage", Property: "nestedPart", Constraint: "deriveUsageNestedPart"},
	{DefiningClass: "Usage", Property: "nestedPort", Constraint: "deriveUsageNestedPort"},
	{DefiningClass: "Usage", Property: "nestedReference", Constraint: "deriveUsageNestedReference"},
	{DefiningClass: "Usage", Property: "nestedRendering", Constraint: "deriveUsageNestedRendering"},
	{DefiningClass: "Usage", Property: "nestedRequirement", Constraint: "deriveUsageNestedRequirement"},
	{DefiningClass: "Usage", Property: "nestedState", Constraint: "deriveUsageNestedState"},
	{DefiningClass: "Usage", Property: "nestedTransition", Constraint: "deriveUsageNestedTransition"},
	{DefiningClass: "Usage", Property: "nestedUsage", Constraint: "deriveUsageNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedUseCase", Constraint: "deriveUsageNestedUseCase"},
	{DefiningClass: "Usage", Property: "nestedVerificationCase", Constraint: "deriveUsageNestedVerificationCase"},
	{DefiningClass: "Usage", Property: "nestedView", Constraint: "deriveUsageNestedView"},
	{DefiningClass: "Usage", Property: "nestedViewpoint", Constraint: "deriveUsageNestedViewpoint"},
	{DefiningClass: "Usage", Property: "ownedUsage", Basis: "redefines Namespace::ownedMember"},
	{DefiningClass: "Usage", Property: "owningDefinition", Basis: "nearest enclosing Definition"},
	{DefiningClass: "Usage", Property: "owningUsage", Basis: "nearest enclosing Usage"},
	{DefiningClass: "Usage", Property: "usage", Constraint: "deriveUsageUsage"},
	{DefiningClass: "Usage", Property: "variant", Constraint: "deriveUsageVariant"},
	{DefiningClass: "Usage", Property: "variantMembership", Constraint: "deriveUsageVariantMembership"},
	{DefiningClass: "Definition", Property: "directedUsage", Constraint: "deriveDefinitionDirectedUsage"},
	{DefiningClass: "Definition", Property: "ownedAction", Constraint: "deriveDefinitionOwnedAction"},
	{DefiningClass: "Definition", Property: "ownedAllocation", Constraint: "deriveDefinitionOwnedAllocation"},
	{DefiningClass: "Definition", Property: "ownedAnalysisCase", Constraint: "deriveDefinitionOwnedAnalysisCase"},
	{DefiningClass: "Definition", Property: "ownedAttribute", Constraint: "deriveDefinitionOwnedAttribute"},
	{DefiningClass: "Definition", Property: "ownedCalculation", Constraint: "deriveDefinitionOwnedCalculation"},
	{DefiningClass: "Definition", Property: "ownedCase", Constraint: "deriveDefinitionOwnedCase"},
	{DefiningClass: "Definition", Property: "ownedConcern", Constraint: "deriveDefinitionOwnedConcern"},
	{DefiningClass: "Definition", Property: "ownedConnection", Constraint: "deriveDefinitionOwnedConnection"},
	{DefiningClass: "Definition", Property: "ownedConstraint", Constraint: "deriveDefinitionOwnedConstraint"},
	{DefiningClass: "Definition", Property: "ownedEnumeration", Constraint: "deriveDefinitionOwnedEnumeration"},
	{DefiningClass: "Definition", Property: "ownedFlow", Constraint: "deriveDefinitionOwnedFlow"},
	{DefiningClass: "Definition", Property: "ownedInterface", Constraint: "deriveDefinitionOwnedInterface"},
	{DefiningClass: "Definition", Property: "ownedItem", Constraint: "deriveDefinitionOwnedItem"},
	{DefiningClass: "Definition", Property: "ownedMetadata", Constraint: "deriveDefinitionOwnedMetadata"},
	{DefiningClass: "Definition", Property: "ownedOccurrence", Constraint: "deriveDefinitionOwnedOccurrence"},
	{DefiningClass: "Definition", Property: "ownedPart", Constraint: "deriveDefinitionOwnedPart"},
	{DefiningClass: "Definition", Property: "ownedPort", Constraint: "deriveDefinitionOwnedPort"},
	{DefiningClass: "Definition", Property: "ownedReference", Constraint: "deriveDefinitionOwnedReference"},
	{DefiningClass: "Definition", Property: "ownedRendering", Constraint: "deriveDefinitionOwnedRendering"},
	{DefiningClass: "Definition", Property: "ownedRequirement", Constraint: "deriveDefinitionOwnedRequirement"},
	{DefiningClass: "Definition", Property: "ownedState", Constraint: "deriveDefinitionOwnedState"},
	{DefiningClass: "Definition", Property: "ownedTransition", Constraint: "deriveDefinitionOwnedTransition"},
	{DefiningClass: "Definition", Property: "ownedUsage", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedUseCase", Constraint: "deriveDefinitionOwnedUseCase"},
	{DefiningClass: "Definition", Property: "ownedVerificationCase", Constraint: "deriveDefinitionOwnedVerificationCase"},
	{DefiningClass: "Definition", Property: "ownedView", Constraint: "deriveDefinitionOwnedView"},
	{DefiningClass: "Definition", Property: "ownedViewpoint", Constraint: "deriveDefinitionOwnedViewpoint"},
	{DefiningClass: "Definition", Property: "usage", Constraint: "deriveDefinitionUsage"},
	{DefiningClass: "Definition", Property: "variant", Constraint: "deriveDefinitionVariant"},
	{DefiningClass: "Definition", Property: "variantMembership", Constraint: "deriveDefinitionVariantMembership"},
}

var ruleConstraints = func() map[string]string {
	out := make(map[string]string, len(servedRules))
	for _, rule := range servedRules {
		out[keyName(rule.DefiningClass, rule.Property)] = rule.Constraint
	}
	return out
}()

const omittedPropertyKeys = `
AcceptActionUsage::payloadArgument
AcceptActionUsage::payloadParameter
AcceptActionUsage::receiverArgument
ActionDefinition::action
ActionUsage::actionDefinition
ActorMembership::ownedActorParameter
AllocationDefinition::allocation
AllocationUsage::allocationDefinition
AnalysisCaseDefinition::resultExpression
AnalysisCaseUsage::analysisCaseDefinition
AnalysisCaseUsage::resultExpression
AnnotatingElement::annotatedElement
AnnotatingElement::annotation
AnnotatingElement::ownedAnnotatingRelationship
AnnotatingElement::owningAnnotatingRelationship
Annotation::annotatingElement
Annotation::ownedAnnotatingElement
Annotation::owningAnnotatedElement
Annotation::owningAnnotatingElement
AssertConstraintUsage::assertedConstraint
AssignmentActionUsage::referent
AssignmentActionUsage::targetArgument
AssignmentActionUsage::valueExpression
Association::associationEnd
Association::relatedType
Association::sourceType
Association::targetType
AttributeUsage::attributeDefinition
Behavior::parameter
Behavior::step
BooleanExpression::predicate
CalculationDefinition::calculation
CalculationUsage::calculationDefinition
CaseDefinition::actorParameter
CaseDefinition::objectiveRequirement
CaseDefinition::subjectParameter
CaseUsage::actorParameter
CaseUsage::caseDefinition
CaseUsage::objectiveRequirement
CaseUsage::subjectParameter
Classifier::ownedSubclassification
ConcernUsage::concernDefinition
ConjugatedPortDefinition::originalPortDefinition
ConjugatedPortDefinition::ownedPortConjugator
ConjugatedPortTyping::portDefinition
Conjugation::owningType
ConnectionDefinition::connectionEnd
ConnectionUsage::connectionDefinition
Connector::association
Connector::connectorEnd
Connector::defaultFeaturingType
Connector::relatedFeature
Connector::sourceFeature
Connector::targetFeature
ConstraintUsage::constraintDefinition
CrossSubsetting::crossingFeature
Differencing::typeDifferenced
Disjoining::owningType
Documentation::documentedElement
ElementFilterMembership::condition
EnumerationDefinition::enumeratedValue
EnumerationUsage::enumerationDefinition
EventOccurrenceUsage::eventOccurrence
ExhibitStateUsage::exhibitedState
Expression::function
Expression::isModelLevelEvaluable
Expression::result
FeatureChainExpression::targetFeature
FeatureChaining::featureChained
FeatureInverting::owningFeature
Feature::endOwningType
Feature::ownedCrossSubsetting
Feature::ownedFeatureChaining
Feature::ownedFeatureInverting
Feature::ownedTypeFeaturing
Feature::owningFeatureMembership
FeatureMembership::ownedMemberFeature
FeatureMembership::owningType
FeatureReferenceExpression::referent
FeatureTyping::owningFeature
FeatureValue::featureWithValue
FeatureValue::value
Flow::flowEnd
Flow::interaction
Flow::payloadFeature
Flow::payloadType
Flow::sourceOutputFeature
Flow::targetInputFeature
FlowDefinition::flowEnd
FlowUsage::flowDefinition
ForLoopActionUsage::loopVariable
ForLoopActionUsage::seqArgument
FramedConcernMembership::ownedConcern
FramedConcernMembership::referencedConcern
Function::expression
Function::isModelLevelEvaluable
Function::result
IfActionUsage::elseAction
IfActionUsage::ifArgument
IfActionUsage::thenAction
Import::importOwningNamespace
Import::importedElement
IncludeUseCaseUsage::useCaseIncluded
InstantiationExpression::argument
InstantiationExpression::instantiatedType
InterfaceDefinition::interfaceEnd
InterfaceUsage::interfaceDefinition
Intersecting::typeIntersected
InvocationExpression::operand
ItemUsage::itemDefinition
LoopActionUsage::bodyAction
Membership::memberElementId
MetadataAccessExpression::referencedElement
MetadataFeature::metaclass
MetadataUsage::metadataDefinition
MultiplicityRange::bound
MultiplicityRange::lowerBound
MultiplicityRange::upperBound
ObjectiveMembership::ownedObjectiveRequirement
OccurrenceUsage::individualDefinition
OccurrenceUsage::occurrenceDefinition
OwningMembership::ownedMemberElement
OwningMembership::ownedMemberElementId
OwningMembership::ownedMemberName
OwningMembership::ownedMemberShortName
Package::filterCondition
ParameterMembership::ownedMemberParameter
PartUsage::partDefinition
PerformActionUsage::performedAction
PortConjugation::conjugatedPortDefinition
PortDefinition::conjugatedPortDefinition
PortUsage::portDefinition
ReferenceSubsetting::referencingFeature
RenderingDefinition::rendering
RenderingUsage::renderingDefinition
RequirementConstraintMembership::ownedConstraint
RequirementConstraintMembership::referencedConstraint
RequirementDefinition::actorParameter
RequirementDefinition::assumedConstraint
RequirementDefinition::framedConcern
RequirementDefinition::requiredConstraint
RequirementDefinition::stakeholderParameter
RequirementDefinition::subjectParameter
RequirementDefinition::text
RequirementUsage::actorParameter
RequirementUsage::assumedConstraint
RequirementUsage::framedConcern
RequirementUsage::requiredConstraint
RequirementUsage::requirementDefinition
RequirementUsage::stakeholderParameter
RequirementUsage::subjectParameter
RequirementUsage::text
RequirementVerificationMembership::ownedRequirement
RequirementVerificationMembership::verifiedRequirement
ResultExpressionMembership::ownedResultExpression
SatisfyRequirementUsage::satisfiedRequirement
SatisfyRequirementUsage::satisfyingFeature
SendActionUsage::payloadArgument
SendActionUsage::receiverArgument
SendActionUsage::senderArgument
Specialization::owningType
StakeholderMembership::ownedStakeholderParameter
StateDefinition::doAction
StateDefinition::entryAction
StateDefinition::exitAction
StateDefinition::state
StateSubactionMembership::action
StateUsage::doAction
StateUsage::entryAction
StateUsage::exitAction
StateUsage::stateDefinition
Step::behavior
Step::parameter
Subclassification::owningClassifier
SubjectMembership::ownedSubjectParameter
Subsetting::owningFeature
TerminateActionUsage::terminatedOccurrenceArgument
TextualRepresentation::representedElement
TransitionFeatureMembership::transitionFeature
TransitionUsage::effectAction
TransitionUsage::guardExpression
TransitionUsage::source
TransitionUsage::succession
TransitionUsage::target
TransitionUsage::triggerAction
Type::ownedConjugator
Type::ownedDifferencing
Type::ownedDisjoining
Type::ownedIntersecting
Type::ownedUnioning
TypeFeaturing::owningFeatureOfType
Unioning::typeUnioned
Usage::mayTimeVary
UseCaseDefinition::includedUseCase
UseCaseUsage::includedUseCase
UseCaseUsage::useCaseDefinition
VariantMembership::ownedVariantUsage
VerificationCaseDefinition::verifiedRequirement
VerificationCaseUsage::verificationCaseDefinition
VerificationCaseUsage::verifiedRequirement
ViewDefinition::satisfiedViewpoint
ViewDefinition::view
ViewDefinition::viewCondition
ViewDefinition::viewRendering
ViewRenderingMembership::ownedRendering
ViewRenderingMembership::referencedRendering
ViewUsage::exposedElement
ViewUsage::satisfiedViewpoint
ViewUsage::viewCondition
ViewUsage::viewDefinition
ViewUsage::viewRendering
ViewpointDefinition::viewpointStakeholder
ViewpointUsage::viewpointDefinition
ViewpointUsage::viewpointStakeholder
WhileLoopActionUsage::untilArgument
WhileLoopActionUsage::whileArgument
`

var omittedProperties = func() []Omission {
	keys := strings.Fields(omittedPropertyKeys)
	omissions := make([]Omission, 0, len(keys))
	for _, key := range keys {
		class, property, _ := strings.Cut(key, "::")
		omissions = append(omissions, Omission{
			DefiningClass: class,
			Property:      property,
			Reason:        omissionReason(class, property),
		})
	}
	return omissions
}()

func omissionReason(class, property string) string {
	switch {
	case strings.Contains(class, "Expression") || strings.HasSuffix(property, "Argument") ||
		strings.HasSuffix(property, "Expression") || property == "condition" || property == "operand":
		return "Requires expression evaluation that the semantic model does not expose."
	case strings.Contains(class, "Membership"):
		return fmt.Sprintf("Requires specialized %s membership selection beyond the current membership handle.", property)
	case class == "Annotation" || class == "AnnotatingElement" || class == "Documentation":
		return fmt.Sprintf("Requires %s annotation-site resolution not exposed by the semantic model.", property)
	case class == "TransitionUsage" || class == "StateUsage" || class == "StateDefinition":
		return fmt.Sprintf("Requires the %s state-machine relation view, which the semantic model does not expose.", property)
	case class == "Connector" || class == "Flow" || class == "Association":
		return fmt.Sprintf("Requires %s connector-end or association resolution not exposed by the semantic model.", property)
	default:
		return fmt.Sprintf("The semantic model does not expose a faithful %s derivation for %s.", property, class)
	}
}

// Rules returns the property derivations the evaluator can faithfully compute.
func Rules() []Rule { return slices.Clone(servedRules) }

// Omitted returns derived properties that require semantic rules not implemented here.
func Omitted() []Omission { return slices.Clone(omittedProperties) }
