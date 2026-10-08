// A connection usage owns its connector ends, named or not, as the
// ReferenceUsages its `connect` clause writes (SysML v2 8.2.2.13.1): they are
// its ownedMember, ownedElement, ownedFeature and ownedEndFeature, and each owns
// the ReferenceSubsetting its attachment states, whose target is the feature
// chain written. A plain usage's `:>>`, `:>` and `::>` clauses are Relationship
// metaobjects it owns the same way (KerML 8.3.4.8.15).
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    port def P { attribute value : Real; }
    part def Source { port y : P; }
    part def Sink { port u : P; }
    connection def C { end source[1] : P; end target[1] : P; }
    part def Asm {
        part s : Source;
        part k : Sink;
        connection c1 : C connect [1] s.y to [1] k.u;
    }
    part def Base {
        attribute x : Real;
        attribute y : Real;
    }
    part def Derived :> Base {
        attribute :>> x = 1.0;
        attribute z : Real :> y;
        ref attribute r ::> y;
    }

    part def Report {
        attribute ownedMembers = (Asm::c1 meta SysML::ConnectionUsage).ownedMember->size();
        attribute ownedElements = (Asm::c1 meta SysML::ConnectionUsage).ownedElement->size();
        attribute ownedFeatures = (Asm::c1 meta SysML::ConnectionUsage).ownedFeature->size();
        ref ends [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature;
        ref endFeatures [*] = (Asm::c1 meta SysML::ConnectionUsage).connectorEnd;
        attribute endsAreMembers = (Asm::c1 meta SysML::ConnectionUsage).ownedMember == (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature;
        ref endOwner [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature#(1).owner;
        ref endReferences [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature.ownedReferenceSubsetting;
        ref endSubsettings [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature.ownedSubsetting;
        ref firstChain [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature#(1).ownedReferenceSubsetting.referencedFeature.chainingFeature;
        ref secondChain [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature#(2).ownedReferenceSubsetting.subsettedFeature.chainingFeature;
        ref referenceSource [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature#(1).ownedReferenceSubsetting.referencingFeature;
        ref referenceTargetOwner [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedEndFeature#(1).ownedReferenceSubsetting.referencedFeature.owner;
        ref connectorSubsettings [*] = (Asm::c1 meta SysML::ConnectionUsage).ownedSubsetting;
        ref redefinitions [*] = (Derived::x meta SysML::AttributeUsage).ownedRedefinition;
        ref redefined [*] = (Derived::x meta SysML::AttributeUsage).ownedRedefinition.redefinedFeature;
        ref redefining [*] = (Derived::x meta SysML::AttributeUsage).ownedRedefinition.redefiningFeature;
        attribute redefinitionIsSubsetting = (Derived::x meta SysML::AttributeUsage).ownedSubsetting == (Derived::x meta SysML::AttributeUsage).ownedRedefinition;
        ref subsetted [*] = (Derived::z meta SysML::AttributeUsage).ownedSubsetting.subsettedFeature;
        ref subsetting [*] = (Derived::z meta SysML::AttributeUsage).ownedSubsetting.subsettingFeature;
        ref noRedefinition [*] = (Derived::z meta SysML::AttributeUsage).ownedRedefinition;
        ref referenced [*] = (Derived::r meta SysML::AttributeUsage).ownedReferenceSubsetting.referencedFeature;
        ref referencing [*] = (Derived::r meta SysML::AttributeUsage).ownedReferenceSubsetting.referencingFeature;
        ref typed [*] = (Derived::z meta SysML::AttributeUsage).ownedTyping.type;
        ref implied [*] = (Derived::z meta SysML::AttributeUsage).ownedTyping.isImplied;
    }
}
