package Model {
    package VehicleExample {
        part def Vehicle {
            doc /* The vehicle under design. */
            part engine : Engine;
        }
        part def Engine;
        connection def vehicle_engine {
            end engine : Engine crosses vehicle.engine;
            end vehicle : Vehicle;
        }
        requirement <R1> MassRequirement {
            doc /* The vehicle shall have a mass of less than 1500 kg. */
        }
        part engine : Engine;
        part vehicle : Vehicle;
        connection 'vehicle_engine 2' : vehicle_engine connect engine to vehicle;
        metadata MigrationMetadata::SynthesizedName about 'vehicle_engine 2';
    }
}
