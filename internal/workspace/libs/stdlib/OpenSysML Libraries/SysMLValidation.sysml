standard library package SysMLValidation {
    doc
    /*
     * Validation constraints of the KerML and SysML abstract syntax, stated over
     * the reflective metamodel (KerML, SysML). Each constraint definition is
     * named for the specification constraint it states and takes the element it
     * checks as its one input, typed by the metaclass the specification declares
     * the constraint on; it applies to every element that metaclass classifies.
     * Constraints whose OCL reads metaclass features the reflective model does
     * not yet derive are not stated here.
     */

    private import ControlFunctions::*;
    private import SequenceFunctions::*;

    /* KerML: Feature */

    constraint def validateFeaturePortionNotVariable {
        doc /* isPortion implies not isVariable */
        in element : KerML::Feature;
        element.isPortion implies not element.isVariable
    }

    constraint def validateFeatureConstantIsVariable {
        doc /* isConstant implies isVariable */
        in element : KerML::Feature;
        element.isConstant implies element.isVariable
    }

    constraint def validateFeatureEndNotDerivedAbstractCompositeOrPortion {
        doc /* isEnd implies not (isDerived or isAbstract or isComposite or isPortion) */
        in element : KerML::Feature;
        element.isEnd implies
            not (element.isDerived or element.isAbstract or element.isComposite or element.isPortion)
    }

    constraint def validateFeatureEndIsConstant {
        doc /* isEnd and isVariable implies isConstant */
        in element : KerML::Feature;
        element.isEnd and element.isVariable implies element.isConstant
    }

    /* SysML: Attributes */

    constraint def validateAttributeUsageIsReference {
        doc /* isReference */
        in element : SysML::AttributeUsage;
        element.isReference
    }

    constraint def validateAttributeUsageFeatures {
        doc /* feature->forAll(not isComposite) */
        in element : SysML::AttributeUsage;
        element.feature->forAll { in f : KerML::Feature; not f.isComposite }
    }

    constraint def validateAttributeDefinitionFeatures {
        doc /* feature->forAll(not isComposite) */
        in element : SysML::AttributeDefinition;
        element.feature->forAll { in f : KerML::Feature; not f.isComposite }
    }

    /* SysML: Enumerations */

    constraint def validateEnumerationDefinitionIsVariation {
        doc /* isVariation */
        in element : SysML::EnumerationDefinition;
        element.isVariation
    }

    /* SysML: References and occurrences */

    constraint def validateReferenceUsageIsReference {
        doc /* isReference */
        in element : SysML::ReferenceUsage;
        element.isReference
    }

    constraint def validateEventOccurrenceUsageIsReference {
        doc /* isReference */
        in element : SysML::EventOccurrenceUsage;
        element.isReference
    }

    /* SysML: Actions */

    constraint def validateControlNodeIsComposite {
        doc /* isComposite */
        in element : SysML::ControlNode;
        element.isComposite
    }

    /* SysML: Ports */

    constraint def validatePortDefinitionOwnedUsagesNotComposite {
        doc /* ownedUsage->reject(oclIsKindOf(PortUsage))->forAll(not isComposite) */
        in element : SysML::PortDefinition;
        element.ownedUsage->reject { in u : SysML::Usage; u istype SysML::PortUsage }
            ->forAll { in u : SysML::Usage; not u.isComposite }
    }

    constraint def validatePortUsageNestedUsagesNotComposite {
        doc /* nestedUsage->reject(oclIsKindOf(PortUsage))->forAll(not isComposite) */
        in element : SysML::PortUsage;
        element.nestedUsage->reject { in u : SysML::Usage; u istype SysML::PortUsage }
            ->forAll { in u : SysML::Usage; not u.isComposite }
    }

    constraint def validateFlowEndIsEnd {
        doc /* isEnd */
        in element : KerML::FlowEnd;
        element.isEnd
    }

    constraint def validateFlowEndNestedFeature {
        doc /* ownedFeature->size() = 1 */
        in element : KerML::FlowEnd;
        size(element.ownedFeature) == 1
    }

    constraint def validateMetadataFeatureMetaclass {
        doc /* type->selectByKind(Metaclass).size() = 1 */
        in element : KerML::MetadataFeature;
        size(element.type->select { in t : KerML::Type; t istype KerML::Metaclass }) == 1
    }

    constraint def validateBindingConnectorIsBinary {
        doc /* relatedFeature->size() = 2 */
        in element : KerML::BindingConnector;
        size(element.relatedFeature) == 2
    }

    constraint def validateConnectorRelatedFeatures {
        doc /* not isAbstract implies relatedFeature->size() >= 2 */
        in element : KerML::Connector;
        not element.isAbstract implies size(element.relatedFeature) >= 2
    }

    constraint def validateFeatureEndNoDirection {
        doc /* isEnd implies direction = null */
        in element : KerML::Feature;
        element.isEnd implies isEmpty(element.direction)
    }

    constraint def validateTypeOwnedMultiplicity {
        doc /* ownedMember->selectByKind(Multiplicity)->size() <= 1 */
        in element : KerML::Type;
        size(element.ownedMember->select { in m : KerML::Element; m istype KerML::Multiplicity }) <= 1
    }

    constraint def validateConnectionDefinitionIsSufficient {
        doc /* isSufficient */
        in element : SysML::ConnectionDefinition;
        element.isSufficient
    }

    constraint def validateDefinitionVariationIsAbstract {
        doc /* isVariation implies isAbstract */
        in element : SysML::Definition;
        element.isVariation implies element.isAbstract
    }

    constraint def validateUsageVariationIsAbstract {
        doc /* isVariation implies isAbstract */
        in element : SysML::Usage;
        element.isVariation implies element.isAbstract
    }

    constraint def validateForLoopActionUsageLoopVariable {
        doc /* ownedFeature->notEmpty() and ownedFeature->at(1).oclIsKindOf(ReferenceUsage) */
        in element : SysML::ForLoopActionUsage;
        notEmpty(element.ownedFeature) and (element.ownedFeature#(1) istype SysML::ReferenceUsage)
    }

    constraint def validateControlNodeOwningType {
        doc /* owningType <> null and (owningType.oclIsKindOf(ActionDefinition) or owningType.oclIsKindOf(ActionUsage)) */
        in element : SysML::ControlNode;
        element.owningType->exists { in t : KerML::Type;
            t istype SysML::ActionDefinition or t istype SysML::ActionUsage }
    }

    constraint def validatePortUsageIsReference {
        doc /* owningType = null or not owningType.oclIsKindOf(PortDefinition) and not owningType.oclIsKindOf(PortUsage) implies isReference */
        in element : SysML::PortUsage;
        (isEmpty(element.owningType) or
            not element.owningType->exists { in t : KerML::Type;
                t istype SysML::PortDefinition or t istype SysML::PortUsage })
        implies element.isReference
    }

    constraint def validateUsageIsReferential {
        doc /* direction <> null or isEnd or featuringType->isEmpty() implies isReference */
        in element : SysML::Usage;
        (notEmpty(element.direction) or element.isEnd or isEmpty(element.featuringType))
        implies element.isReference
    }

    constraint def validateCaseDefinitionSubjectParameterPosition {
        doc /* input->notEmpty() and input->first() = subjectParameter */
        in element : SysML::CaseDefinition;
        notEmpty(element.input) and element.input#(1) == element.subjectParameter
    }

    constraint def validateCaseUsageSubjectParameterPosition {
        doc /* input->notEmpty() and input->first() = subjectParameter */
        in element : SysML::CaseUsage;
        notEmpty(element.input) and element.input#(1) == element.subjectParameter
    }

    constraint def validateRequirementDefinitionSubjectParameterPosition {
        doc /* input->notEmpty() and input->first() = subjectParameter */
        in element : SysML::RequirementDefinition;
        notEmpty(element.input) and element.input#(1) == element.subjectParameter
    }

    constraint def validateRequirementUsageSubjectParameterPosition {
        doc /* input->notEmpty() and input->first() = subjectParameter */
        in element : SysML::RequirementUsage;
        notEmpty(element.input) and element.input#(1) == element.subjectParameter
    }

    constraint def validateOccurrenceUsageIsPortion {
        doc /* portionKind <> null implies isPortion */
        in element : SysML::OccurrenceUsage;
        notEmpty(element.portionKind) implies element.isPortion
    }

    constraint def validateOccurrenceUsagePortionKind {
        doc /* portionKind <> null implies owningType <> null and (owningType.oclIsKindOf(OccurrenceDefinition) or owningType.oclIsKindOf(OccurrenceUsage)) */
        in element : SysML::OccurrenceUsage;
        notEmpty(element.portionKind) implies
            element.owningType->exists { in t : KerML::Type;
                t istype SysML::OccurrenceDefinition or t istype SysML::OccurrenceUsage }
    }

    constraint def validateOccurrenceUsageIndividualDefinition {
        doc /* occurrenceDefinition->selectByKind(OccurrenceDefinition)->select(isIndividual).size() <= 1 */
        in element : SysML::OccurrenceUsage;
        size(element.occurrenceDefinition->select { in d : KerML::Class;
            d istype SysML::OccurrenceDefinition and (d meta SysML::OccurrenceDefinition).isIndividual }) <= 1
    }

    constraint def validateOccurrenceUsageIndividualUsage {
        doc /* isIndividual implies individualDefinition <> null */
        in element : SysML::OccurrenceUsage;
        element.isIndividual implies notEmpty(element.individualDefinition)
    }

    constraint def validatePartUsagePartDefinition {
        doc /* partDefinition->notEmpty() */
        in element : SysML::PartUsage;
        notEmpty(element.partDefinition)
    }

    constraint def validateTypeUnioningTypesNotSelf {
        doc /* unioningType->excludes(self) */
        in element : KerML::Type;
        excludes(element.unioningType, element)
    }

    constraint def validateTypeIntersectingTypesNotSelf {
        doc /* intersectingType->excludes(self) */
        in element : KerML::Type;
        excludes(element.intersectingType, element)
    }

    constraint def validateTypeDifferencingTypesNotSelf {
        doc /* differencingType->excludes(self) */
        in element : KerML::Type;
        excludes(element.differencingType, element)
    }

    constraint def validateAssociationEndTypes {
        doc /* ownedEndFeature->forAll(type->size() = 1) */
        in element : KerML::Association;
        element.ownedEndFeature->forAll { in f : KerML::Feature; size(f.type) == 1 }
    }

    constraint def validateMetadataFeatureMetaclassNotAbstract {
        doc /* not metaclass.isAbstract */
        in element : KerML::MetadataFeature;
        element.metaclass->forAll { in c : KerML::Metaclass; not c.isAbstract }
    }
}
