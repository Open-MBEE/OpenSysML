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
	{DefiningClass: "Classifier", Property: "ownedSubclassification", Constraint: "deriveClassifierOwnedSubclassification"},
	{DefiningClass: "Annotation", Property: "owningAnnotatedElement", Basis: "opposite of Element::ownedAnnotation"},
	{DefiningClass: "Annotation", Property: "annotatingElement", Constraint: "deriveAnnotationAnnotatingElement"},
	{DefiningClass: "Annotation", Property: "ownedAnnotatingElement", Constraint: "deriveAnnotationOwnedAnnotatingElement"},
	{DefiningClass: "Annotation", Property: "owningAnnotatingElement", Basis: "subset of Annotation::owningRelatedElement"},
	{DefiningClass: "AnnotatingElement", Property: "annotatedElement", Constraint: "deriveAnnotatingElementAnnotatedElement"},
	{DefiningClass: "AnnotatingElement", Property: "annotation", Constraint: "deriveAnnotatingElementAnnotation"},
	{DefiningClass: "AnnotatingElement", Property: "ownedAnnotatingRelationship", Constraint: "deriveAnnotatingElementOwnedAnnotatingRelationship"},
	{DefiningClass: "Documentation", Property: "documentedElement", Basis: "owning namespace of the Documentation"},
	{DefiningClass: "Conjugation", Property: "owningType", Basis: "opposite of Type::ownedConjugator"},
	{DefiningClass: "Disjoining", Property: "owningType", Basis: "opposite of Type::ownedDisjoining"},
	{DefiningClass: "Differencing", Property: "owningType", Basis: "opposite of Type::ownedDifferencing"},
	{DefiningClass: "FeatureTyping", Property: "owningFeature", Basis: "opposite of Feature::ownedTyping"},
	{DefiningClass: "FeatureInverting", Property: "owningFeature", Basis: "opposite of Feature::ownedFeatureInverting"},
	{DefiningClass: "CrossSubsetting", Property: "owningFeature", Basis: "opposite of Feature::ownedCrossSubsetting"},
	{DefiningClass: "FeatureChaining", Property: "owningFeature", Basis: "opposite of Feature::ownedFeatureChaining"},
	{DefiningClass: "FeatureMembership", Property: "owningType", Basis: "owning namespace of the FeatureMembership"},
	{DefiningClass: "FeatureMembership", Property: "ownedMemberFeature", Basis: "memberElement when it is a Feature"},
	{DefiningClass: "Import", Property: "importOwningNamespace", Basis: "owning related namespace"},
	{DefiningClass: "FeatureValue", Property: "featureWithValue", Basis: "owning membership namespace"},
	{DefiningClass: "FeatureValue", Property: "value", Basis: "member element of the FeatureValue membership"},
	{DefiningClass: "Behavior", Property: "step", Constraint: "deriveBehaviorStep"},
	{DefiningClass: "Behavior", Property: "parameter", Basis: "redefines Type::directedFeature"},
	{DefiningClass: "Step", Property: "behavior", Constraint: "deriveStepBehavior"},
	{DefiningClass: "Step", Property: "parameter", Basis: "owned ParameterMembership members of the Step"},
	{DefiningClass: "Expression", Property: "function", Basis: "function type of the expression"},
	{DefiningClass: "Expression", Property: "result", Constraint: "deriveExpressionResult"},
	{DefiningClass: "InstantiationExpression", Property: "instantiatedType", Constraint: "deriveInstantiationExpressionInstantiatedType"},
	{DefiningClass: "Function", Property: "expression", Basis: "Expression-valued Function::step"},
	{DefiningClass: "Function", Property: "result", Constraint: "deriveFunctionResult"},
	{DefiningClass: "ActionUsage", Property: "actionDefinition", Basis: "Behavior-valued typing of the ActionUsage"},
	{DefiningClass: "AllocationUsage", Property: "allocationDefinition", Basis: "AllocationDefinition-valued typing of the AllocationUsage"},
	{DefiningClass: "AnalysisCaseUsage", Property: "analysisCaseDefinition", Basis: "AnalysisCaseDefinition-valued typing of the AnalysisCaseUsage"},
	{DefiningClass: "AttributeUsage", Property: "attributeDefinition", Basis: "DataType-valued typing of the AttributeUsage"},
	{DefiningClass: "CalculationUsage", Property: "calculationDefinition", Basis: "Function-valued typing of the CalculationUsage"},
	{DefiningClass: "CaseUsage", Property: "caseDefinition", Basis: "CaseDefinition-valued typing of the CaseUsage"},
	{DefiningClass: "ConcernUsage", Property: "concernDefinition", Basis: "ConcernDefinition-valued typing of the ConcernUsage"},
	{DefiningClass: "ConnectionUsage", Property: "connectionDefinition", Basis: "AssociationStructure-valued typing of the ConnectionUsage"},
	{DefiningClass: "ConstraintUsage", Property: "constraintDefinition", Basis: "Predicate-valued typing of the ConstraintUsage"},
	{DefiningClass: "EnumerationUsage", Property: "enumerationDefinition", Basis: "EnumerationDefinition-valued typing of the EnumerationUsage"},
	{DefiningClass: "FlowUsage", Property: "flowDefinition", Basis: "Interaction-valued typing of the FlowUsage"},
	{DefiningClass: "InterfaceUsage", Property: "interfaceDefinition", Basis: "InterfaceDefinition-valued typing of the InterfaceUsage"},
	{DefiningClass: "MetadataUsage", Property: "metadataDefinition", Basis: "Metaclass-valued typing of the MetadataUsage"},
	{DefiningClass: "OccurrenceUsage", Property: "occurrenceDefinition", Basis: "Class-valued typing of the OccurrenceUsage"},
	{DefiningClass: "PartUsage", Property: "partDefinition", Constraint: "derivePartUsagePartDefinition"},
	{DefiningClass: "PortUsage", Property: "portDefinition", Basis: "PortDefinition-valued typing of the PortUsage"},
	{DefiningClass: "RenderingUsage", Property: "renderingDefinition", Basis: "RenderingDefinition-valued typing of the RenderingUsage"},
	{DefiningClass: "RequirementUsage", Property: "requirementDefinition", Basis: "RequirementDefinition-valued typing of the RequirementUsage"},
	{DefiningClass: "StateUsage", Property: "stateDefinition", Basis: "Behavior-valued typing of the StateUsage"},
	{DefiningClass: "UseCaseUsage", Property: "useCaseDefinition", Basis: "UseCaseDefinition-valued typing of the UseCaseUsage"},
	{DefiningClass: "VerificationCaseUsage", Property: "verificationCaseDefinition", Basis: "VerificationCaseDefinition-valued typing of the VerificationCaseUsage"},
	{DefiningClass: "ViewUsage", Property: "viewDefinition", Basis: "ViewDefinition-valued typing of the ViewUsage"},
	{DefiningClass: "ViewpointUsage", Property: "viewpointDefinition", Basis: "ViewpointDefinition-valued typing of the ViewpointUsage"},
	{DefiningClass: "CaseDefinition", Property: "actorParameter", Constraint: "deriveCaseDefinitionActorParameter"},
	{DefiningClass: "CaseDefinition", Property: "objectiveRequirement", Constraint: "deriveCaseDefinitionObjectiveRequirement"},
	{DefiningClass: "CaseDefinition", Property: "subjectParameter", Constraint: "deriveCaseDefinitionSubjectParameter"},
	{DefiningClass: "CaseUsage", Property: "actorParameter", Constraint: "deriveCaseUsageActorParameter"},
	{DefiningClass: "CaseUsage", Property: "objectiveRequirement", Constraint: "deriveCaseUsageObjectiveRequirement"},
	{DefiningClass: "CaseUsage", Property: "subjectParameter", Constraint: "deriveCaseUsageSubjectParameter"},
	{DefiningClass: "RequirementDefinition", Property: "actorParameter", Constraint: "deriveRequirementDefinitionActorParameter"},
	{DefiningClass: "RequirementDefinition", Property: "assumedConstraint", Constraint: "deriveRequirementDefinitionAssumedConstraint"},
	{DefiningClass: "RequirementDefinition", Property: "framedConcern", Constraint: "deriveRequirementDefinitionFramedConcern"},
	{DefiningClass: "RequirementDefinition", Property: "requiredConstraint", Constraint: "deriveRequirementDefinitionRequiredConstraint"},
	{DefiningClass: "RequirementDefinition", Property: "stakeholderParameter", Constraint: "deriveRequirementDefinitionStakeholderParameter"},
	{DefiningClass: "RequirementDefinition", Property: "subjectParameter", Constraint: "deriveRequirementDefinitionSubjectParameter"},
	{DefiningClass: "RequirementDefinition", Property: "text", Constraint: "deriveRequirementDefinitionText"},
	{DefiningClass: "RequirementUsage", Property: "assumedConstraint", Constraint: "deriveRequirementUsageAssumedConstraint"},
	{DefiningClass: "RequirementUsage", Property: "framedConcern", Constraint: "deriveRequirementUsageFramedConcern"},
	{DefiningClass: "RequirementUsage", Property: "requiredConstraint", Constraint: "deriveRequirementUsageRequiredConstraint"},
	{DefiningClass: "RequirementUsage", Property: "actorParameter", Constraint: "deriveRequirementUsageActorParameter"},
	{DefiningClass: "RequirementUsage", Property: "stakeholderParameter", Constraint: "deriveRequirementUsageStakeholderParameter"},
	{DefiningClass: "RequirementUsage", Property: "subjectParameter", Constraint: "deriveRequirementUsageSubjectParameter"},
	{DefiningClass: "StateDefinition", Property: "doAction", Constraint: "deriveStateDefinitionDoAction"},
	{DefiningClass: "StateDefinition", Property: "entryAction", Constraint: "deriveStateDefinitionEntryAction"},
	{DefiningClass: "StateDefinition", Property: "exitAction", Constraint: "deriveStateDefinitionExitAction"},
	{DefiningClass: "StateDefinition", Property: "state", Constraint: "deriveStateDefinitionState"},
	{DefiningClass: "StateUsage", Property: "doAction", Constraint: "deriveStateUsageDoAction"},
	{DefiningClass: "StateUsage", Property: "entryAction", Constraint: "deriveStateUsageEntryAction"},
	{DefiningClass: "StateUsage", Property: "exitAction", Constraint: "deriveStateUsageExitAction"},
	{DefiningClass: "Connector", Property: "relatedFeature", Constraint: "deriveConnectorRelatedFeature"},
	{DefiningClass: "Connector", Property: "sourceFeature", Constraint: "deriveConnectorSourceFeature"},
	{DefiningClass: "Connector", Property: "targetFeature", Constraint: "deriveConnectorTargetFeature"},
	{DefiningClass: "Connector", Property: "connectorEnd", Basis: "ConnectorEnd features of the connector"},
	{DefiningClass: "Connector", Property: "association", Basis: "Association types of the connector"},
	{DefiningClass: "Connector", Property: "defaultFeaturingType", Constraint: "deriveConnectorDefaultFeaturingType"},
	{DefiningClass: "MultiplicityRange", Property: "bound", Constraint: "deriveMultiplicityRangeBound"},
	{DefiningClass: "MultiplicityRange", Property: "lowerBound", Constraint: "deriveMultiplicityRangeLowerBound"},
	{DefiningClass: "MultiplicityRange", Property: "upperBound", Constraint: "deriveMultiplicityRangeUpperBound"},
	{DefiningClass: "Import", Property: "importedElement", Basis: "MembershipImport and NamespaceImport imported-element derivations"},
	{DefiningClass: "Membership", Property: "memberElementId", Basis: "writer emits elementId of memberElement"},
	{DefiningClass: "OwningMembership", Property: "ownedMemberElementId", Basis: "elementId of ownedMemberElement"},
	{DefiningClass: "OwningMembership", Property: "ownedMemberName", Constraint: "deriveOwningMembershipOwnedMemberName"},
	{DefiningClass: "OwningMembership", Property: "ownedMemberShortName", Constraint: "deriveOwningMembershipOwnedMemberShortName"},
	{DefiningClass: "Feature", Property: "chainingFeature", Constraint: "deriveFeatureChainingFeature"},
	{DefiningClass: "Feature", Property: "crossFeature", Constraint: "deriveFeatureCrossFeature"},
	{DefiningClass: "Feature", Property: "direction", Basis: "owned attribute; value set by the textual notation"},
	{DefiningClass: "Feature", Property: "endOwningType", Basis: "opposite of Type::ownedEndFeature"},
	{DefiningClass: "Feature", Property: "featureTarget", Constraint: "deriveFeatureFeatureTarget"},
	{DefiningClass: "Feature", Property: "featuringType", Constraint: "deriveFeatureFeaturingType"},
	{DefiningClass: "Feature", Property: "isComposite", Basis: "owned attribute; value set by the textual notation"},
	{DefiningClass: "Feature", Property: "isVariable", Basis: "KerML variable/constant declaration or Usage::mayTimeVary"},
	{DefiningClass: "Feature", Property: "ownedCrossSubsetting", Constraint: "deriveFeatureOwnedCrossSubsetting"},
	{DefiningClass: "Feature", Property: "ownedFeatureChaining", Constraint: "deriveFeatureOwnedFeatureChaining"},
	{DefiningClass: "Feature", Property: "ownedFeatureInverting", Constraint: "deriveFeatureOwnedFeatureInverting"},
	{DefiningClass: "Feature", Property: "ownedRedefinition", Constraint: "deriveFeatureOwnedRedefinition"},
	{DefiningClass: "Feature", Property: "ownedReferenceSubsetting", Constraint: "deriveFeatureOwnedReferenceSubsetting"},
	{DefiningClass: "Feature", Property: "ownedSubsetting", Constraint: "deriveFeatureOwnedSubsetting"},
	{DefiningClass: "Feature", Property: "ownedTypeFeaturing", Constraint: "deriveFeatureOwnedTypeFeaturing"},
	{DefiningClass: "Feature", Property: "ownedTyping", Constraint: "deriveFeatureOwnedTyping"},
	{DefiningClass: "Feature", Property: "owningFeatureMembership", Basis: "opposite of FeatureMembership::ownedMemberFeature"},
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
	{DefiningClass: "Annotation", Property: "annotatedElement", Basis: "redefines Relationship::target"},
	{DefiningClass: "Relationship", Property: "ownedRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "owningRelatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "relatedElement", Constraint: "deriveRelationshipRelatedElement"},
	{DefiningClass: "Relationship", Property: "source", Basis: "first endpoint in the relationship's relatedElement order"},
	{DefiningClass: "Relationship", Property: "target", Basis: "second endpoint in the relationship's relatedElement order"},
	{DefiningClass: "Specialization", Property: "owningType", Basis: "opposite of Type::ownedSpecialization"},
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
	{DefiningClass: "Type", Property: "ownedConjugator", Constraint: "deriveTypeOwnedConjugator"},
	{DefiningClass: "Type", Property: "ownedDifferencing", Constraint: "deriveTypeOwnedDifferencing"},
	{DefiningClass: "Type", Property: "ownedDisjoining", Constraint: "deriveTypeOwnedDisjoining"},
	{DefiningClass: "Type", Property: "ownedIntersecting", Constraint: "deriveTypeOwnedIntersecting"},
	{DefiningClass: "Type", Property: "ownedUnioning", Constraint: "deriveTypeOwnedUnioning"},
	{DefiningClass: "Intersecting", Property: "owningType", Basis: "opposite of Type::ownedIntersecting"},
	{DefiningClass: "Unioning", Property: "owningType", Basis: "opposite of Type::ownedUnioning"},
	{DefiningClass: "Type", Property: "unioningType", Constraint: "deriveTypeUnioningType"},
	{DefiningClass: "Usage", Property: "definition", Basis: "redefines Feature::type"},
	{DefiningClass: "Usage", Property: "directedUsage", Constraint: "deriveUsageDirectedUsage"},
	{DefiningClass: "Usage", Property: "isReference", Constraint: "deriveUsageIsReference"},
	{DefiningClass: "Usage", Property: "mayTimeVary", Basis: "Occurrence ownership, portion status, and composite/Action classification"},
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
	{DefiningClass: "Usage", Property: "owningDefinition", Basis: "owningType when it is a Definition"},
	{DefiningClass: "Usage", Property: "owningUsage", Basis: "owningType when it is a Usage"},
	{DefiningClass: "Usage", Property: "usage", Constraint: "deriveUsageUsage"},
	{DefiningClass: "Usage", Property: "variant", Constraint: "deriveUsageVariant"},
	{DefiningClass: "Usage", Property: "variantMembership", Constraint: "deriveUsageVariantMembership"},
	{DefiningClass: "ItemUsage", Property: "itemDefinition", Constraint: "deriveItemUsageItemDefinition"},
	{DefiningClass: "OccurrenceUsage", Property: "individualDefinition", Constraint: "deriveOccurrenceUsageIndividualDefinition"},
	{DefiningClass: "UseCaseDefinition", Property: "includedUseCase", Constraint: "deriveUseCaseDefinitionIncludedUseCase"},
	{DefiningClass: "TransitionUsage", Property: "guardExpression", Constraint: "deriveTransitionUsageGuardExpression"},
	{DefiningClass: "TransitionUsage", Property: "source", Constraint: "deriveTransitionUsageSource"},
	{DefiningClass: "TransitionUsage", Property: "succession", Constraint: "deriveTransitionUsageSuccession"},
	{DefiningClass: "TransitionUsage", Property: "target", Constraint: "deriveTransitionUsageTarget"},
	{DefiningClass: "TransitionUsage", Property: "triggerAction", Constraint: "deriveTransitionUsageTriggerAction"},
	{DefiningClass: "TransitionUsage", Property: "effectAction", Constraint: "deriveTransitionUsageEffectAction"},
	{DefiningClass: "RequirementConstraintMembership", Property: "referencedConstraint", Constraint: "deriveRequirementConstraintMembershipReferencedConstraint"},
	{DefiningClass: "Flow", Property: "flowEnd", Constraint: "deriveFlowFlowEnd"},
	{DefiningClass: "Flow", Property: "payloadFeature", Constraint: "deriveFlowPayloadFeature"},
	{DefiningClass: "Flow", Property: "payloadType", Constraint: "deriveFlowPayloadType"},
	{DefiningClass: "Flow", Property: "sourceOutputFeature", Constraint: "deriveFlowSourceOutputFeature"},
	{DefiningClass: "Flow", Property: "targetInputFeature", Constraint: "deriveFlowTargetInputFeature"},
	{DefiningClass: "Flow", Property: "interaction", Basis: "Interaction-valued typing of the Flow"},
	{DefiningClass: "ConjugatedPortTyping", Property: "portDefinition", Constraint: "deriveConjugatedPortTypingPortDefinition"},
	{DefiningClass: "ConjugatedPortTyping", Property: "conjugatedPortDefinition", Basis: "redefines FeatureTyping::type as a ConjugatedPortDefinition"},
	{DefiningClass: "PortDefinition", Property: "conjugatedPortDefinition", Constraint: "derivePortDefinitionConjugatedPortDefinition"},
	{DefiningClass: "ConjugatedPortDefinition", Property: "originalPortDefinition", Basis: "owningNamespace of the ConjugatedPortDefinition"},
	{DefiningClass: "PortConjugation", Property: "originalPortDefinition", Basis: "source endpoint of PortConjugation"},
	{DefiningClass: "PortConjugation", Property: "conjugatedPortDefinition", Basis: "target endpoint of PortConjugation"},
	{DefiningClass: "RequirementConstraintMembership", Property: "ownedConstraint", Basis: "memberElement of RequirementConstraintMembership"},
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
		if _, served := ruleConstraints[key]; served {
			continue
		}
		omissions = append(omissions, Omission{
			DefiningClass: class,
			Property:      property,
			Reason:        omissionReason(class, property),
		})
	}
	return omissions
}()

func omissionReason(class, property string) string {
	key := class + "::" + property
	switch {
	case property == "lowerBound" || property == "upperBound" || property == "bound":
		return fmt.Sprintf("Requires the %s engine fact: evaluate the referenced multiplicity-bound Expression to an integer.", key)
	case property == "isModelLevelEvaluable":
		return fmt.Sprintf("Requires the %s engine fact: evaluate the expression or function body and determine whether its result is model-level.", key)
	case strings.Contains(class, "Expression") || strings.HasSuffix(property, "Argument") ||
		strings.HasSuffix(property, "Expression") || property == "condition" || property == "operand":
		return fmt.Sprintf("Requires the %s engine fact: evaluate the AST expression and resolve its resulting Feature or value.", key)
	case strings.Contains(class, "Membership"):
		return fmt.Sprintf("Requires the %s engine fact: the relationship's actual membership metaclass and its typed memberElement endpoint.", key)
	case class == "Annotation" || class == "AnnotatingElement" || class == "Documentation":
		return fmt.Sprintf("Requires the %s engine fact: the annotation/documentation relationship endpoint and its owning element.", key)
	case class == "TransitionUsage" || class == "StateUsage" || class == "StateDefinition":
		return fmt.Sprintf("Requires the %s engine fact: the state-machine source, target, trigger, effect, or subaction endpoint selected for this property.", key)
	case class == "Connector" || class == "Flow" || class == "Association":
		return fmt.Sprintf("Requires the %s engine fact: the connector-end membership, its referenced Feature, and the associated Type needed by this property.", key)
	default:
		return fmt.Sprintf("Requires the %s engine fact: a resolved graph relationship or semantic value for this property, which the current semantic index does not provide.", key)
	}
}

// Rules returns the property derivations the evaluator can faithfully compute.
func Rules() []Rule { return slices.Clone(servedRules) }

// Omitted returns derived properties that require semantic rules not implemented here.
func Omitted() []Omission { return slices.Clone(omittedProperties) }
