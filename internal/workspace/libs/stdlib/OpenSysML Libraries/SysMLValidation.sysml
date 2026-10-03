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
}
