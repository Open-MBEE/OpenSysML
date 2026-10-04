package metamodel

import (
	"fmt"
	"slices"
	"strings"
)

var servedRules = []Rule{
	{DefiningClass: "Element", Property: "documentation", Constraint: "deriveElementDocumentation"},
	{DefiningClass: "Element", Property: "isImpliedIncluded", Constraint: "implied elements are not emitted"},
	{DefiningClass: "Element", Property: "isLibraryElement", Constraint: "deriveLibraryElement"},
	{DefiningClass: "Element", Property: "name", Constraint: "deriveEffectiveName"},
	{DefiningClass: "Element", Property: "ownedAnnotation", Constraint: "deriveOwnedAnnotation"},
	{DefiningClass: "Element", Property: "ownedElement", Constraint: "deriveOwnedElement"},
	{DefiningClass: "Element", Property: "ownedRelationship", Constraint: "deriveOwnedRelationship"},
	{DefiningClass: "Element", Property: "owner", Constraint: "deriveOwner"},
	{DefiningClass: "Element", Property: "owningMembership", Constraint: "deriveOwningMembership"},
	{DefiningClass: "Element", Property: "owningNamespace", Constraint: "deriveOwningNamespace"},
	{DefiningClass: "Element", Property: "qualifiedName", Constraint: "deriveQualifiedName"},
	{DefiningClass: "Element", Property: "shortName", Constraint: "deriveEffectiveShortName"},
	{DefiningClass: "Element", Property: "textualRepresentation", Constraint: "deriveTextualRepresentation"},
	{DefiningClass: "Feature", Property: "chainingFeature", Constraint: "deriveFeatureChaining"},
	{DefiningClass: "Feature", Property: "crossFeature", Constraint: "deriveCrossFeature"},
	{DefiningClass: "Feature", Property: "direction", Constraint: "deriveEffectiveDirection"},
	{DefiningClass: "Feature", Property: "featureTarget", Constraint: "deriveFeatureTarget"},
	{DefiningClass: "Feature", Property: "featuringType", Constraint: "deriveFeaturingType"},
	{DefiningClass: "Feature", Property: "isComposite", Constraint: "deriveIsComposite"},
	{DefiningClass: "Feature", Property: "ownedRedefinition", Constraint: "deriveOwnedRedefinition"},
	{DefiningClass: "Feature", Property: "ownedReferenceSubsetting", Constraint: "deriveOwnedReferenceSubsetting"},
	{DefiningClass: "Feature", Property: "ownedSubsetting", Constraint: "deriveOwnedSubsetting"},
	{DefiningClass: "Feature", Property: "ownedTyping", Constraint: "deriveOwnedTyping"},
	{DefiningClass: "Feature", Property: "owningType", Constraint: "deriveOwningType"},
	{DefiningClass: "Feature", Property: "type", Constraint: "deriveFeatureType"},
	{DefiningClass: "Membership", Property: "memberElement", Constraint: "deriveMembershipMemberElement"},
	{DefiningClass: "Membership", Property: "memberName", Constraint: "deriveMembershipMemberName"},
	{DefiningClass: "Membership", Property: "memberShortName", Constraint: "deriveMembershipMemberShortName"},
	{DefiningClass: "Membership", Property: "membershipOwningNamespace", Constraint: "deriveMembershipOwningNamespace"},
	{DefiningClass: "Namespace", Property: "importedMembership", Constraint: "deriveNamespaceImport"},
	{DefiningClass: "Namespace", Property: "member", Constraint: "deriveNamespaceMembership"},
	{DefiningClass: "Namespace", Property: "membership", Constraint: "deriveNamespaceMembership"},
	{DefiningClass: "Namespace", Property: "ownedImport", Constraint: "deriveNamespaceImport"},
	{DefiningClass: "Namespace", Property: "ownedMember", Constraint: "deriveNamespaceMembership"},
	{DefiningClass: "Namespace", Property: "ownedMembership", Constraint: "deriveNamespaceMembership"},
	{DefiningClass: "Relationship", Property: "ownedRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "owningRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "relatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "source", Constraint: "deriveRelationshipSource"},
	{DefiningClass: "Relationship", Property: "target", Constraint: "deriveRelationshipTarget"},
	{DefiningClass: "Type", Property: "differencingType", Constraint: "deriveTypeDifferencing"},
	{DefiningClass: "Type", Property: "directedFeature", Constraint: "deriveTypeDirectedFeature"},
	{DefiningClass: "Type", Property: "endFeature", Constraint: "deriveTypeEndFeature"},
	{DefiningClass: "Type", Property: "feature", Constraint: "deriveTypeFeature"},
	{DefiningClass: "Type", Property: "featureMembership", Constraint: "deriveTypeFeatureMembership"},
	{DefiningClass: "Type", Property: "inheritedFeature", Constraint: "deriveTypeInheritedMembership"},
	{DefiningClass: "Type", Property: "inheritedMembership", Constraint: "deriveTypeInheritedMembership"},
	{DefiningClass: "Type", Property: "input", Constraint: "deriveTypeDirectedFeature"},
	{DefiningClass: "Type", Property: "intersectingType", Constraint: "deriveTypeIntersecting"},
	{DefiningClass: "Type", Property: "isConjugated", Constraint: "deriveTypeConjugation"},
	{DefiningClass: "Type", Property: "multiplicity", Constraint: "deriveTypeMultiplicity"},
	{DefiningClass: "Type", Property: "output", Constraint: "deriveTypeDirectedFeature"},
	{DefiningClass: "Type", Property: "ownedEndFeature", Constraint: "deriveTypeEndFeature"},
	{DefiningClass: "Type", Property: "ownedFeature", Constraint: "deriveTypeFeature"},
	{DefiningClass: "Type", Property: "ownedFeatureMembership", Constraint: "deriveTypeFeatureMembership"},
	{DefiningClass: "Type", Property: "ownedSpecialization", Constraint: "deriveTypeSpecialization"},
	{DefiningClass: "Type", Property: "unioningType", Constraint: "deriveTypeUnioning"},
	{DefiningClass: "Usage", Property: "definition", Constraint: "deriveUsageDefinition"},
	{DefiningClass: "Usage", Property: "directedUsage", Constraint: "deriveUsage"},
	{DefiningClass: "Usage", Property: "isReference", Constraint: "deriveUsageReference"},
	{DefiningClass: "Usage", Property: "nestedAction", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedAllocation", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedAnalysisCase", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedAttribute", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedCalculation", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedCase", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedConcern", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedConnection", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedConstraint", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedEnumeration", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedFlow", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedInterface", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedItem", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedMetadata", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedOccurrence", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedPart", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedPort", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedReference", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedRendering", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedRequirement", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedState", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedTransition", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedUsage", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedUseCase", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedVerificationCase", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedView", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "nestedViewpoint", Constraint: "deriveNestedUsage"},
	{DefiningClass: "Usage", Property: "owningDefinition", Constraint: "deriveOwningDefinition"},
	{DefiningClass: "Usage", Property: "owningUsage", Constraint: "deriveOwningUsage"},
	{DefiningClass: "Usage", Property: "usage", Constraint: "deriveUsage"},
	{DefiningClass: "Usage", Property: "variant", Constraint: "deriveUsageVariant"},
	{DefiningClass: "Usage", Property: "variantMembership", Constraint: "deriveUsageVariant"},
	{DefiningClass: "Definition", Property: "directedUsage", Constraint: "deriveUsage"},
	{DefiningClass: "Definition", Property: "ownedAction", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedAllocation", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedAnalysisCase", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedAttribute", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedCalculation", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedCase", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedConcern", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedConnection", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedConstraint", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedEnumeration", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedFlow", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedInterface", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedItem", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedMetadata", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedOccurrence", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedPart", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedPort", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedReference", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedRendering", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedRequirement", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedState", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedTransition", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedUsage", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedUseCase", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedVerificationCase", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedView", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "ownedViewpoint", Constraint: "deriveDefinitionOwnedUsage"},
	{DefiningClass: "Definition", Property: "usage", Constraint: "deriveUsage"},
	{DefiningClass: "Definition", Property: "variant", Constraint: "deriveUsageVariant"},
	{DefiningClass: "Definition", Property: "variantMembership", Constraint: "deriveUsageVariant"},
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
