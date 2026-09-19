// A concern is a requirement, so the conditions a viewpoint frames evaluate
// against an element a view exposes exactly as a requirement's do — including
// the ones the framed concern usage inherits from its concern definition.
package test {
    part def Vehicle {
        attribute mass;
        attribute partCount;
    }

    concern def MassBudget {
        subject vehicle : Vehicle;
        attribute maxMass = 2000.0;
        require constraint {
            vehicle.mass <= maxMass
        }
    }

    concern def StructureConcern :> MassBudget {
        attribute maxParts;
        require constraint {
            vehicle.partCount <= maxParts
        }
    }

    viewpoint def StructurePerspective {
        frame concern structure : StructureConcern {
            attribute :>> maxMass = 1500.0;
            attribute :>> maxParts = 3;
        }
    }

    part lightVehicle : Vehicle {
        attribute :>> mass = 1200.0;
        attribute :>> partCount = 2;
    }

    part heavyVehicle : Vehicle {
        attribute :>> mass = 1800.0;
        attribute :>> partCount = 2;
    }

    part complexVehicle : Vehicle {
        attribute :>> mass = 1200.0;
        attribute :>> partCount = 7;
    }

    part analysisContext {
        assert satisfy StructurePerspective::structure by lightVehicle;
        assert satisfy StructurePerspective::structure by heavyVehicle;
        assert satisfy StructurePerspective::structure by complexVehicle;
    }
}
