package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// apiJSONCompletedPropertiesKey names, on an element object, the derived
// properties the writer completed from the metamodel rather than read from the graph.
const apiJSONCompletedPropertiesKey = "sysx:completedProperties"

// apiJSONCompletedProperty is a derived property of the metamodel the element
// form completes from the evaluator when the graph states nothing for it.
type apiJSONCompletedProperty struct {
	class string
	name  string
	// clause cites the derive* constraint the value follows (KerML 1.1 Beta 2 §8.3, SysML 2.1 Beta 2 §8.4).
	clause string
	// defaultValue completes a non-derived attribute at its metamodel default when set.
	defaultValue any
}

// apiJSONCompletedProperties lists the completed properties in the order the
// element form writes them. A class's entry applies to every metaclass under it.
var apiJSONCompletedProperties = []apiJSONCompletedProperty{
	{class: "Element", name: "isLibraryElement", clause: "8.3.2.1.2 deriveElementIsLibraryElement"},
	{class: "Membership", name: "memberElementId", clause: "8.3.2.4.3 deriveMembershipMemberElementId"},
	{class: "OwningMembership", name: "ownedMemberElementId", clause: "8.3.2.4.8 ownedMemberElementId redefines memberElementId"},
	{class: "OwningMembership", name: "ownedMemberName", clause: "8.3.2.4.8 deriveOwningMembershipOwnedMemberName"},
	{class: "OwningMembership", name: "ownedMemberShortName", clause: "8.3.2.4.8 deriveOwningMembershipOwnedMemberShortName"},
	{class: "Membership", name: "visibility", clause: "8.3.2.4.3 visibility : VisibilityKind = public", defaultValue: "public"},
	{class: "Import", name: "visibility", clause: "8.3.2.4.2 visibility : VisibilityKind = private", defaultValue: "private"},
	{class: "Element", name: "name", clause: "8.3.2.1.2 deriveElementName"},
	{class: "Element", name: "shortName", clause: "8.3.2.1.2 deriveElementShortName"},
	{class: "Element", name: "owningMembership", clause: "8.3.2.1.2 deriveElementOwningMembership"},
	{class: "Element", name: "owningNamespace", clause: "8.3.2.1.2 deriveElementOwningNamespace"},
	{class: "Element", name: "ownedElement", clause: "8.3.2.1.2 deriveElementOwnedElement"},
	{class: "Element", name: "documentation", clause: "8.3.2.1.2 deriveElementDocumentation"},
	{class: "Element", name: "textualRepresentation", clause: "8.3.2.1.2 deriveElementTextualRepresentation"},
	{class: "Namespace", name: "ownedMembership", clause: "8.3.2.4.5 deriveNamespaceOwnedMembership"},
	{class: "Namespace", name: "ownedMember", clause: "8.3.2.4.5 deriveNamespaceOwnedMember"},
	{class: "Namespace", name: "ownedImport", clause: "8.3.2.4.5 deriveNamespaceOwnedImport"},
	{class: "Namespace", name: "importedMembership", clause: "8.3.2.4.5 deriveNamespaceImportedMembership"},
	{class: "AnnotatingElement", name: "annotatedElement", clause: "8.3.2.3.2 deriveAnnotatingElementAnnotatedElement"},
	{class: "AnnotatingElement", name: "annotation", clause: "8.3.2.3.2 deriveAnnotatingElementAnnotation"},
	{class: "AnnotatingElement", name: "ownedAnnotatingRelationship", clause: "8.3.2.3.2 deriveAnnotatingElementOwnedAnnotatingRelationship"},
	{class: "Documentation", name: "documentedElement", clause: "8.3.2.3.5 deriveDocumentationDocumentedElement"},
	{class: "Type", name: "isConjugated", clause: "8.3.3.1.10 deriveTypeIsConjugated"},
	{class: "Type", name: "ownedSpecialization", clause: "8.3.3.1.10 deriveTypeOwnedSpecialization"},
	{class: "Type", name: "ownedFeatureMembership", clause: "8.3.3.1.10 deriveTypeOwnedFeatureMembership"},
	{class: "Type", name: "ownedFeature", clause: "8.3.3.1.10 deriveTypeOwnedFeature"},
	{class: "Type", name: "ownedEndFeature", clause: "8.3.3.1.10 deriveTypeOwnedEndFeature"},
	{class: "Type", name: "ownedConjugator", clause: "8.3.3.1.10 deriveTypeOwnedConjugator"},
	{class: "Type", name: "multiplicity", clause: "8.3.3.1.10 deriveTypeMultiplicity"},
	{class: "Namespace", name: "membership", clause: "8.3.2.4.5 deriveNamespaceMembership"},
	{class: "Namespace", name: "member", clause: "8.3.2.4.5 deriveNamespaceMember"},
	{class: "Type", name: "inheritedMembership", clause: "8.3.3.1.10 deriveTypeInheritedMembership"},
	{class: "Type", name: "featureMembership", clause: "8.3.3.1.10 deriveTypeFeatureMembership"},
	{class: "Type", name: "feature", clause: "8.3.3.1.10 deriveTypeFeature"},
	{class: "Type", name: "inheritedFeature", clause: "8.3.3.1.10 deriveTypeInheritedFeature"},
	{class: "Type", name: "directedFeature", clause: "8.3.3.1.10 deriveTypeDirectedFeature"},
	{class: "Type", name: "input", clause: "8.3.3.1.10 deriveTypeInput"},
	{class: "Type", name: "output", clause: "8.3.3.1.10 deriveTypeOutput"},
	{class: "Usage", name: "usage", clause: "SysML 8.3.6.4 deriveUsageUsage"},
	{class: "Usage", name: "directedUsage", clause: "SysML 8.3.6.4 deriveUsageDirectedUsage"},
	{class: "Definition", name: "usage", clause: "SysML 8.3.6.2 deriveDefinitionUsage"},
	{class: "Definition", name: "directedUsage", clause: "SysML 8.3.6.2 deriveDefinitionDirectedUsage"},
	{class: "Classifier", name: "ownedSubclassification", clause: "8.3.3.2.2 deriveClassifierOwnedSubclassification"},
	{class: "Feature", name: "owningType", clause: "8.3.3.3.4 deriveFeatureOwningType"},
	{class: "Feature", name: "owningFeatureMembership", clause: "8.3.3.3.4 deriveFeatureOwningFeatureMembership"},
	{class: "Feature", name: "endOwningType", clause: "8.3.3.3.4 deriveFeatureEndOwningType"},
	{class: "Feature", name: "featureTarget", clause: "8.3.3.3.4 deriveFeatureFeatureTarget"},
	{class: "Feature", name: "featuringType", clause: "8.3.3.3.4 deriveFeatureFeaturingType"},
	{class: "Feature", name: "type", clause: "8.3.3.3.4 deriveFeatureType"},
	{class: "Feature", name: "ownedTyping", clause: "8.3.3.3.4 deriveFeatureOwnedTyping"},
	{class: "Feature", name: "ownedSubsetting", clause: "8.3.3.3.4 deriveFeatureOwnedSubsetting"},
	{class: "Feature", name: "ownedRedefinition", clause: "8.3.3.3.4 deriveFeatureOwnedRedefinition"},
	{class: "Feature", name: "ownedReferenceSubsetting", clause: "8.3.3.3.4 deriveFeatureOwnedReferenceSubsetting"},
	{class: "Feature", name: "ownedTypeFeaturing", clause: "8.3.3.3.4 deriveFeatureOwnedTypeFeaturing"},
	{class: "Feature", name: "ownedFeatureChaining", clause: "8.3.3.3.4 deriveFeatureOwnedFeatureChaining"},
	{class: "Feature", name: "chainingFeature", clause: "8.3.3.3.4 deriveFeatureChainingFeature"},
	{class: "FeatureMembership", name: "owningType", clause: "8.3.3.1.6 owningType redefines membershipOwningNamespace"},
	{class: "Specialization", name: "owningType", clause: "8.3.3.1.8 owningType"},
	{class: "Conjugation", name: "owningType", clause: "8.3.3.1.2 owningType"},
	{class: "Disjoining", name: "owningType", clause: "8.3.3.1.4 owningType"},
	{class: "Differencing", name: "owningType", clause: "8.3.3.1.3 owningType"},
	{class: "Intersecting", name: "owningType", clause: "8.3.3.1.7 owningType"},
	{class: "Unioning", name: "owningType", clause: "8.3.3.1.11 owningType"},
	{class: "Step", name: "behavior", clause: "8.3.4.6.3 deriveStepBehavior"},
	{class: "Expression", name: "function", clause: "8.3.4.7.3 deriveExpressionFunction"},
	{class: "Expression", name: "result", clause: "8.3.4.7.3 deriveExpressionResult"},
	{class: "MultiplicityRange", name: "bound", clause: "8.3.4.11.2 deriveMultiplicityRangeBound"},
	{class: "Usage", name: "owningDefinition", clause: "SysML 8.3.6.4 deriveUsageOwningDefinition"},
	{class: "Usage", name: "owningUsage", clause: "SysML 8.3.6.4 deriveUsageOwningUsage"},
	{class: "Usage", name: "definition", clause: "SysML 8.3.6.4 deriveUsageDefinition"},
	{class: "Usage", name: "nestedUsage", clause: "SysML 8.3.6.4 deriveUsageNestedUsage"},
	{class: "Usage", name: "nestedOccurrence", clause: "SysML 8.3.6.4 deriveUsageNestedOccurrence"},
	{class: "Usage", name: "nestedReference", clause: "SysML 8.3.6.4 deriveUsageNestedReference"},
	{class: "Usage", name: "nestedAttribute", clause: "SysML 8.3.6.4 deriveUsageNestedAttribute"},
	{class: "Usage", name: "nestedItem", clause: "SysML 8.3.6.4 deriveUsageNestedItem"},
	{class: "Usage", name: "nestedPart", clause: "SysML 8.3.6.4 deriveUsageNestedPart"},
	{class: "Usage", name: "nestedPort", clause: "SysML 8.3.6.4 deriveUsageNestedPort"},
	{class: "Usage", name: "nestedAction", clause: "SysML 8.3.6.4 deriveUsageNestedAction"},
	{class: "Usage", name: "nestedState", clause: "SysML 8.3.6.4 deriveUsageNestedState"},
	{class: "Usage", name: "nestedConstraint", clause: "SysML 8.3.6.4 deriveUsageNestedConstraint"},
	{class: "Usage", name: "nestedCalculation", clause: "SysML 8.3.6.4 deriveUsageNestedCalculation"},
	{class: "Usage", name: "nestedConnection", clause: "SysML 8.3.6.4 deriveUsageNestedConnection"},
	{class: "Usage", name: "nestedMetadata", clause: "SysML 8.3.6.4 deriveUsageNestedMetadata"},
	{class: "Usage", name: "nestedRequirement", clause: "SysML 8.3.6.4 deriveUsageNestedRequirement"},
	{class: "Usage", name: "variant", clause: "SysML 8.3.6.4 deriveUsageVariant"},
	{class: "Usage", name: "variantMembership", clause: "SysML 8.3.6.4 deriveUsageVariantMembership"},
	{class: "Definition", name: "ownedUsage", clause: "SysML 8.3.6.2 deriveDefinitionOwnedUsage"},
	{class: "Definition", name: "ownedOccurrence", clause: "SysML 8.3.6.2 deriveDefinitionOwnedOccurrence"},
	{class: "Definition", name: "ownedReference", clause: "SysML 8.3.6.2 deriveDefinitionOwnedReference"},
	{class: "Definition", name: "ownedAttribute", clause: "SysML 8.3.6.2 deriveDefinitionOwnedAttribute"},
	{class: "Definition", name: "ownedItem", clause: "SysML 8.3.6.2 deriveDefinitionOwnedItem"},
	{class: "Definition", name: "ownedPart", clause: "SysML 8.3.6.2 deriveDefinitionOwnedPart"},
	{class: "Definition", name: "ownedPort", clause: "SysML 8.3.6.2 deriveDefinitionOwnedPort"},
	{class: "Definition", name: "ownedAction", clause: "SysML 8.3.6.2 deriveDefinitionOwnedAction"},
	{class: "Definition", name: "ownedState", clause: "SysML 8.3.6.2 deriveDefinitionOwnedState"},
	{class: "Definition", name: "ownedConstraint", clause: "SysML 8.3.6.2 deriveDefinitionOwnedConstraint"},
	{class: "Definition", name: "ownedCalculation", clause: "SysML 8.3.6.2 deriveDefinitionOwnedCalculation"},
	{class: "Definition", name: "ownedConnection", clause: "SysML 8.3.6.2 deriveDefinitionOwnedConnection"},
	{class: "Definition", name: "ownedMetadata", clause: "SysML 8.3.6.2 deriveDefinitionOwnedMetadata"},
	{class: "Definition", name: "variant", clause: "SysML 8.3.6.2 deriveDefinitionVariant"},
	{class: "Definition", name: "variantMembership", clause: "SysML 8.3.6.2 deriveDefinitionVariantMembership"},
	{class: "AttributeUsage", name: "attributeDefinition", clause: "SysML 8.3.7.2 deriveAttributeUsageAttributeDefinition"},
	{class: "OccurrenceUsage", name: "occurrenceDefinition", clause: "SysML 8.3.9.4 deriveOccurrenceUsageOccurrenceDefinition"},
	{class: "ItemUsage", name: "itemDefinition", clause: "SysML 8.3.10.3 deriveItemUsageItemDefinition"},
	{class: "PartUsage", name: "partDefinition", clause: "SysML 8.3.11.3 derivePartUsagePartDefinition"},
	{class: "PortUsage", name: "portDefinition", clause: "SysML 8.3.12.5 derivePortUsagePortDefinition"},
	{class: "ActionUsage", name: "actionDefinition", clause: "SysML 8.3.17.4 deriveActionUsageActionDefinition"},
	{class: "StateUsage", name: "stateDefinition", clause: "SysML 8.3.18.6 deriveStateUsageStateDefinition"},
	{class: "ConstraintUsage", name: "constraintDefinition", clause: "SysML 8.3.20.4 deriveConstraintUsageConstraintDefinition"},
	{class: "CalculationUsage", name: "calculationDefinition", clause: "SysML 8.3.19.3 deriveCalculationUsageCalculationDefinition"},
	{class: "ConnectionUsage", name: "connectionDefinition", clause: "SysML 8.3.13.4 deriveConnectionUsageConnectionDefinition"},
	{class: "MetadataUsage", name: "metadataDefinition", clause: "SysML 8.3.27.4 deriveMetadataUsageMetadataDefinition"},
	{class: "RequirementUsage", name: "requirementDefinition", clause: "SysML 8.3.21.9 deriveRequirementUsageRequirementDefinition"},
	{class: "RequirementUsage", name: "subjectParameter", clause: "SysML 8.3.21.9 deriveRequirementUsageSubjectParameter"},
	{class: "RequirementUsage", name: "text", clause: "SysML 8.3.21.9 deriveRequirementUsageText"},
	{class: "RequirementUsage", name: "requiredConstraint", clause: "SysML 8.3.21.9 deriveRequirementUsageRequiredConstraint"},
	{class: "RequirementUsage", name: "assumedConstraint", clause: "SysML 8.3.21.9 deriveRequirementUsageAssumedConstraint"},
	{class: "RequirementUsage", name: "actorParameter", clause: "SysML 8.3.21.9 deriveRequirementUsageActorParameter"},
	{class: "RequirementUsage", name: "stakeholderParameter", clause: "SysML 8.3.21.9 deriveRequirementUsageStakeholderParameter"},
	{class: "RequirementUsage", name: "framedConcern", clause: "SysML 8.3.21.9 deriveRequirementUsageFramedConcern"},
	{class: "RequirementDefinition", name: "subjectParameter", clause: "SysML 8.3.21.8 deriveRequirementDefinitionSubjectParameter"},
	{class: "RequirementDefinition", name: "text", clause: "SysML 8.3.21.8 deriveRequirementDefinitionText"},
	{class: "RequirementDefinition", name: "requiredConstraint", clause: "SysML 8.3.21.8 deriveRequirementDefinitionRequiredConstraint"},
	{class: "RequirementDefinition", name: "assumedConstraint", clause: "SysML 8.3.21.8 deriveRequirementDefinitionAssumedConstraint"},
	{class: "RequirementDefinition", name: "actorParameter", clause: "SysML 8.3.21.8 deriveRequirementDefinitionActorParameter"},
	{class: "RequirementDefinition", name: "stakeholderParameter", clause: "SysML 8.3.21.8 deriveRequirementDefinitionStakeholderParameter"},
	{class: "RequirementDefinition", name: "framedConcern", clause: "SysML 8.3.21.8 deriveRequirementDefinitionFramedConcern"},
	{class: "RequirementConstraintMembership", name: "referencedConstraint", clause: "SysML 8.3.21.7 deriveRequirementConstraintMembershipReferencedConstraint"},
	{class: "CaseUsage", name: "subjectParameter", clause: "SysML 8.3.22.3 deriveCaseUsageSubjectParameter"},
	{class: "CaseUsage", name: "actorParameter", clause: "SysML 8.3.22.3 deriveCaseUsageActorParameter"},
	{class: "CaseDefinition", name: "subjectParameter", clause: "SysML 8.3.22.2 deriveCaseDefinitionSubjectParameter"},
	{class: "CaseDefinition", name: "actorParameter", clause: "SysML 8.3.22.2 deriveCaseDefinitionActorParameter"},
	{class: "StateUsage", name: "entryAction", clause: "SysML 8.3.18.6 deriveStateUsageEntryAction"},
	{class: "StateUsage", name: "doAction", clause: "SysML 8.3.18.6 deriveStateUsageDoAction"},
	{class: "StateUsage", name: "exitAction", clause: "SysML 8.3.18.6 deriveStateUsageExitAction"},
	{class: "StateDefinition", name: "entryAction", clause: "SysML 8.3.18.5 deriveStateDefinitionEntryAction"},
	{class: "StateDefinition", name: "doAction", clause: "SysML 8.3.18.5 deriveStateDefinitionDoAction"},
	{class: "StateDefinition", name: "exitAction", clause: "SysML 8.3.18.5 deriveStateDefinitionExitAction"},
	{class: "AcceptActionUsage", name: "payloadArgument", clause: "SysML 8.3.17.2 deriveAcceptActionUsagePayloadArgument"},
	{class: "SendActionUsage", name: "payloadArgument", clause: "SysML 8.3.17.15 deriveSendActionUsagePayloadArgument"},
	{class: "SendActionUsage", name: "senderArgument", clause: "SysML 8.3.17.15 deriveSendActionUsageSenderArgument"},
	{class: "Flow", name: "flowEnd", clause: "8.3.4.9.2 deriveFlowFlowEnd"},
	{class: "Flow", name: "payloadFeature", clause: "8.3.4.9.2 deriveFlowPayloadFeature"},
	{class: "Flow", name: "payloadType", clause: "8.3.4.9.2 deriveFlowPayloadType"},
}

// apiJSONCompletedPropertyNames maps every completed property name to the
// classes that complete it, for the reader and the compact form.
var apiJSONCompletedPropertyNames = func() map[string][]string {
	out := make(map[string][]string, len(apiJSONCompletedProperties))
	for _, property := range apiJSONCompletedProperties {
		out[property.name] = append(out[property.name], property.class)
	}
	return out
}()

// apiJSONCompletesProperty reports whether the element form completes name on
// an element of metaclass.
func apiJSONCompletesProperty(metaclass, name string) bool {
	for _, class := range apiJSONCompletedPropertyNames[name] {
		if ontology.IsAncestorOrSelf(metaclass, class) {
			return true
		}
	}
	return false
}

// apiJSONMetaclassCompletion is a completed property with its graph predicate,
// listed once per metaclass.
type apiJSONMetaclassCompletion struct {
	apiJSONCompletedProperty
	predicate string
}

var apiJSONCompletionsByMetaclass sync.Map

// apiJSONCompletedPropertiesOf lists the completed properties a metaclass has,
// computed once per metaclass from the table.
func apiJSONCompletedPropertiesOf(metaclass string) []apiJSONMetaclassCompletion {
	if cached, ok := apiJSONCompletionsByMetaclass.Load(metaclass); ok {
		return cached.([]apiJSONMetaclassCompletion)
	}
	var completions []apiJSONMetaclassCompletion
	for _, property := range apiJSONCompletedProperties {
		if ontology.IsAncestorOrSelf(metaclass, property.class) {
			completions = append(completions, apiJSONMetaclassCompletion{property, rdf.SysML + property.name})
		}
	}
	cached, _ := apiJSONCompletionsByMetaclass.LoadOrStore(metaclass, completions)
	return cached.([]apiJSONMetaclassCompletion)
}

// completeDerived appends the completed derived properties of subject the graph
// does not state, and the marker naming them.
func (w *apiJSONElementWriter) completeDerived(subject rdf.Term, metaclass string, element apiJSONObject) apiJSONObject {
	if metaclass == "" || w.evaluation == nil || w.evaluation.evaluator == nil || w.evaluation.structure == nil ||
		LibraryReference(w.graph, subject) || apiJSONExternalForm(w.graph, w.scratch) {
		return element
	}
	semanticElement, ok := w.evaluation.structure.element(subject)
	if !ok {
		return element
	}
	var completed []any
	for _, property := range apiJSONCompletedPropertiesOf(metaclass) {
		if w.graph.HasProperty(subject, property.predicate) || apiJSONHasMember(element, property.name) {
			continue
		}
		spelled := property.defaultValue
		if spelled == nil {
			value, supported := w.evaluation.evaluator.PropertyReadOnlyUncached(semanticElement, property.class, property.name)
			if !supported {
				reason, explained := w.evaluation.evaluator.Explain(semanticElement, property.class, property.name)
				if !explained {
					reason = metamodel.Reason{Category: metamodel.ReasonNoRule, Detail: property.class + "::" + property.name}
				}
				w.completionOmitted(subject, metaclass, property.name, reason)
				continue
			}
			var ok bool
			if spelled, ok = w.derivedJSONValue(value); !ok {
				if !derivedValueEmpty(value) {
					w.completionOmitted(subject, metaclass, property.name, metamodel.Reason{
						Category: metamodel.ReasonUnrepresentedElement, Detail: property.class + "::" + property.name,
					})
				}
				continue
			}
			// A single-valued property is spelled as its one value, as the graph's would be.
			if definition, known := ontology.PropertyOf(metaclass, property.name); known && !definition.Many {
				values, sequence := spelled.([]any)
				if sequence && len(values) != 1 {
					w.completionOmitted(subject, metaclass, property.name, metamodel.Reason{
						Category: metamodel.ReasonIncompleteStructure,
						Detail:   fmt.Sprintf("%d values for the single-valued %s::%s", len(values), property.class, property.name),
					})
					continue
				}
				if sequence {
					spelled = values[0]
				}
			}
		}
		element = append(element, apiJSONMember{key: property.name, value: spelled})
		completed = append(completed, property.name)
	}
	if len(completed) > 0 {
		element = append(element, apiJSONMember{key: apiJSONCompletedPropertiesKey, value: completed})
	}
	return element
}

// completionOmitted reports a completed property the subject's element object
// does not carry, and why, to the writer's Omitted option.
func (w *apiJSONElementWriter) completionOmitted(subject rdf.Term, metaclass, property string, reason metamodel.Reason) {
	if w.options.Omitted == nil {
		return
	}
	id, _ := rdf.SubjectID(subject)
	w.options.Omitted(APIJSONOmission{Subject: id, Metaclass: metaclass, Property: property, Reason: reason})
}

// derivedValueEmpty reports a value the element form spells by omission: null,
// or an empty collection.
func derivedValueEmpty(value metamodel.Value) bool {
	return value.Kind == metamodel.NullValue || (value.Kind == metamodel.SequenceValue && len(value.Values) == 0)
}

// derivedJSONValue spells an evaluator value in the element form; a null or
// empty value, or one naming an element no id addresses, is not spelled.
func (w *apiJSONElementWriter) derivedJSONValue(value metamodel.Value) (any, bool) {
	switch value.Kind {
	case metamodel.BooleanValue:
		return value.Boolean, true
	case metamodel.StringValue:
		return value.String, true
	case metamodel.EnumValue:
		return value.Enum, true
	case metamodel.IntegerValue:
		return value.Integer, true
	case metamodel.RealValue:
		return value.Real, true
	case metamodel.ElementValue:
		id, ok := w.derivedElementID(value.Element)
		if !ok {
			return nil, false
		}
		return APIJSONReference{ID: id}, true
	case metamodel.MembershipValue:
		id, ok := w.derivedElementID(metamodel.MembershipElement(value.Membership))
		if !ok {
			return nil, false
		}
		return APIJSONReference{ID: id}, true
	case metamodel.SequenceValue:
		if len(value.Values) == 0 {
			return nil, false
		}
		values := make([]any, 0, len(value.Values))
		for _, member := range value.Values {
			spelled, ok := w.derivedJSONValue(member)
			if !ok {
				return nil, false
			}
			values = append(values, spelled)
		}
		return values, true
	}
	return nil, false
}

// derivedElementID is the "@id" of an element the evaluator names: a graph
// subject's id, or a library element's normative id.
func (w *apiJSONElementWriter) derivedElementID(element metamodel.Element) (string, bool) {
	if subject, ok := w.evaluation.structure.subject(element); ok {
		return rdf.SubjectID(subject)
	}
	symbol := element.Symbol
	if element.IsMembership {
		symbol = element.Membership.Member
	}
	if symbol == nil {
		return "", false
	}
	library, ok := w.evaluation.libraryCatalog().ElementForSymbol(symbol)
	if !ok || library == nil {
		return w.anonymousLibraryID(symbol, element.IsMembership)
	}
	if element.IsMembership {
		return library.OwningMembershipID, library.OwningMembershipID != ""
	}
	return library.ID, library.ID != ""
}

// anonymousLibraryID mints the id of a library member no qualified name
// catalogues (identity.AnonymousMemberID), under its catalogued owner.
func (w *apiJSONElementWriter) anonymousLibraryID(symbol *symbols.Symbol, membership bool) (string, bool) {
	if w.evaluation.model == nil {
		return "", false
	}
	member, ok := sharedLibraryIndex().anonymous.Member(symbol)
	if !ok {
		member, ok = identity.AnonymousMemberOf(w.evaluation.model, w.evaluation.libraryCatalog(), symbol)
	}
	if !ok {
		return "", false
	}
	if membership {
		return member.OwningMembershipID, member.OwningMembershipID != ""
	}
	return member.ID, member.ID != ""
}

// libraryCatalog is the catalog of normative library ids, built once per evaluation.
func (e *apiJSONEvaluation) libraryCatalog() *identity.Catalog {
	return e.structure.libraryCatalog()
}

// apiJSONCheckCompletedProperties refuses a sysx:completedProperties list naming a
// property the writer does not complete for the metaclass; a stated one is never dropped on its word.
func apiJSONCheckCompletedProperties(object *apiJSONElementData, metaclass string) error {
	for name := range object.completedProperties {
		if !apiJSONCompletesProperty(metaclass, name) {
			return fmt.Errorf("element %q: %s names %q, which the element form does not complete on a %s",
				object.id, apiJSONCompletedPropertiesKey, name, object.typ)
		}
	}
	return nil
}
