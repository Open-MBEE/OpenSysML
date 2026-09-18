package MetadataPrefixedConstraints {
    metadata def goal;
    metadata def stakeholderConcern;

    attribute massValue = 1.0;

    constraint def MassLimit;

    constraint payloadMassLimit { massValue < 2.0 }

    requirement vehicleMassRequirement { require constraint { massValue < 3.0 } }

    #goal requirement deliverPayload {
        assume #goal constraint payloadMassLimit;
        require #goal vehicleMassRequirement;
        require #goal #stakeholderConcern constraint massMargin : MassLimit;
        assume #goal constraint { massValue > 0.0 }
    }
}
