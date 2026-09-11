// `x meta T` (KerML 7.4.9.2 MetaCastExpression) is `x.metadata as T`: of the
// element x names, the annotations whose metadata type conforms to T in model
// order, then its reflective metaobject when its own metaclass conforms to T
// (KerML 8.3.4.8.15); the empty sequence when neither does. The metaobject's
// features are the metaclass's, read from the element's declaration.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    metadata def Safety {
        attribute level : Integer = 2;
    }

    part def Vehicle {
        attribute mass : Real;
        part wheel : Wheel[4];
    }

    part def Wheel;

    part seatBelt : Vehicle {
        @Safety { level = 4; }
    }

    abstract part def Shape;

    part def Report {
        attribute usage [*] = seatBelt meta SysML::PartUsage;
        attribute asFeature [*] = seatBelt meta KerML::Feature;
        attribute definitionName = (seatBelt meta SysML::PartUsage).definition.declaredName;
        attribute notADefinition [*] = seatBelt meta SysML::PartDefinition;
        attribute definition [*] = Vehicle meta SysML::PartDefinition;
        attribute safety [*] = seatBelt meta Safety;
        attribute safetyLevel = (seatBelt meta Safety).level;
        attribute noSafety [*] = Vehicle meta Safety;
        attribute anyMetadata [*] = seatBelt meta Metaobjects::Metaobject;
        attribute name = (seatBelt meta KerML::Feature).declaredName;
        attribute qualifiedName = (seatBelt meta KerML::Feature).qualifiedName;
        attribute typeName = (seatBelt meta KerML::Feature).type.declaredName;
        attribute ownedFeatures = (Vehicle meta KerML::Type).ownedFeature->size();
        attribute ownedMemberNames [*] = (Vehicle meta KerML::Namespace).ownedMember.declaredName;
        attribute abstract = (Shape meta KerML::Type).isAbstract;
        attribute concrete = (Vehicle meta KerML::Type).isAbstract;
        attribute direction [*] = (seatBelt meta KerML::Feature).direction;
        attribute sameElement = (seatBelt meta KerML::Type) === (seatBelt meta KerML::Feature);
        attribute sameAgain = (seatBelt meta KerML::Feature) == (seatBelt meta KerML::Feature);
        attribute otherElement = (seatBelt meta KerML::Feature) === (Vehicle meta KerML::Type);
    }
}
