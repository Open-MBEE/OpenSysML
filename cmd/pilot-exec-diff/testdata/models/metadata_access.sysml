package Meta {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    metadata def Safety {
        attribute isMandatory : Boolean;
        attribute level : Integer default 2;
    }

    part def Vehicle;
    part def Wheel {
        doc /* Turns. */
    }

    part seatBelt : Vehicle {
        @Safety {
            isMandatory = true;
            level = 4;
        }
    }

    part chassis : Vehicle {
        part wheel : Wheel;
        attribute mass : Real = 1.5;
        rep asJava language "Java" /* class Chassis {} */
    }

    dependency relies from seatBelt to Vehicle;

    attribute metadataCount = seatBelt.metadata->size();
    attribute bareMetadataCount = chassis.metadata->size();
    attribute metaFeatureCount = (seatBelt meta KerML::Feature)->size();
    attribute metaSafetyCount = (seatBelt meta Safety)->size();
    attribute metaSafetyLevel = (seatBelt meta Safety).level;
    attribute metaName = (seatBelt meta KerML::Feature).name;
    attribute metaDeclaredName = (chassis meta KerML::Feature).declaredName;
    attribute metaQualifiedName = (chassis meta KerML::Feature).qualifiedName;
    attribute metaOwnedFeatureCount = (chassis meta KerML::Type).ownedFeature->size();
    attribute metaDefinitionCount = (chassis meta SysML::PartDefinition)->size();
    attribute metaUsageCount = (chassis meta SysML::PartUsage)->size();
    attribute metaDefinitionName = (Vehicle meta SysML::PartDefinition).declaredName;
    attribute metaIsAbstract = (Vehicle meta KerML::Type).isAbstract;
    attribute metaSame = (chassis meta KerML::Feature) === (chassis meta KerML::Type);
    attribute metaDocCount = (Wheel meta KerML::Element).documentation->size();
    attribute metaDocBody = (Wheel meta KerML::Element).documentation.body;
    attribute metaDocQualifiedName = (Wheel meta KerML::Element).documentation.qualifiedName;
    attribute metaDocOwnerName = (Wheel meta KerML::Element).documentation.owner.declaredName;
    attribute metaNoDocCount = (Vehicle meta KerML::Element).documentation->size();
    attribute dependencyMetadataCount = relies.metadata->size();
    attribute metaDependencyCount = (relies meta KerML::Dependency)->size();
    attribute metaDependencyClientName = (relies meta KerML::Dependency).client.declaredName;
    attribute repMetadataCount = chassis::asJava.metadata->size();
    attribute metaRepRepresentedName = (chassis::asJava meta KerML::TextualRepresentation).representedElement.declaredName;
}
