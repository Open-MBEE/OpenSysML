package Model {
    package Pumping {
        part def Pump;
        requirement <R1> 'Flow Rate' {
            doc /* The pump shall deliver 10 L/min. */
        }
        occurrence def Gauge {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "Block";
                profile = "http:///Acme/Stereotypes.ecore";
            }
        }
    }
}
