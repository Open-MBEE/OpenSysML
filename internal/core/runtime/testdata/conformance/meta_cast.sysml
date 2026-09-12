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
        rep asJava language "Java" /* class Vehicle {} */
    }

    part def Wheel {
        doc /* Turns. */
        comment locale "en" /* Four per vehicle. */
    }

    part seatBelt : Vehicle {
        @Safety { level = 4; }
    }

    abstract part def Shape {
        doc /**/
    }

    dependency relies from seatBelt to Vehicle, Wheel;

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
        attribute docs [*] = (Wheel meta KerML::Element).documentation;
        attribute docBodies [*] = (Wheel meta KerML::Element).documentation.body;
        attribute docOwner = (Wheel meta KerML::Element).documentation#(1).owner === (Wheel meta KerML::Type)#(1);
        attribute docQualifiedName [*] = (Wheel meta KerML::Element).documentation.qualifiedName;
        attribute noDocs [*] = (Vehicle meta KerML::Element).documentation;
        attribute blankDocBody [*] = (Shape meta KerML::Element).documentation.body;
        attribute commentLocales [*] = (Wheel meta KerML::Namespace).ownedMember.locale;
        attribute dependencyMetadata [*] = relies.metadata;
        attribute dependency [*] = relies meta KerML::Element;
        attribute notAFeature [*] = relies meta KerML::Feature;
        attribute clients [*] = (relies meta KerML::Dependency).client.declaredName;
        attribute suppliers [*] = (relies meta KerML::Relationship).target.qualifiedName;
        attribute repMetadata [*] = Vehicle::asJava.metadata;
        attribute rep [*] = Vehicle::asJava meta KerML::AnnotatingElement;
        attribute repLanguage = (Vehicle::asJava meta KerML::TextualRepresentation).language;
        attribute repBody = (Vehicle::asJava meta KerML::TextualRepresentation).body;
        attribute repRepresents = (Vehicle::asJava meta KerML::TextualRepresentation)#(1).representedElement === (Vehicle meta KerML::Type)#(1);
        attribute sameElement = (seatBelt meta KerML::Type) === (seatBelt meta KerML::Feature);
        attribute sameAgain = (seatBelt meta KerML::Feature) == (seatBelt meta KerML::Feature);
        attribute otherElement = (seatBelt meta KerML::Feature) === (Vehicle meta KerML::Type);
    }
}
