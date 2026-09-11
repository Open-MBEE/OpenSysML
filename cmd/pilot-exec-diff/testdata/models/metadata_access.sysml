package Meta {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    metadata def Safety {
        attribute isMandatory : Boolean;
        attribute level : Integer default 2;
    }

    part def Vehicle;
    part def Wheel;

    part seatBelt : Vehicle {
        @Safety {
            isMandatory = true;
            level = 4;
        }
    }

    part chassis : Vehicle {
        part wheel : Wheel;
        attribute mass : Real = 1.5;
    }

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
}
