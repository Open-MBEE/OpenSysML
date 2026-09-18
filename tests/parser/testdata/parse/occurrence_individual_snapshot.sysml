package IndividualsAndSnapshots {
    occurrence def Flight;
    part def Vehicle;
    individual def TestSystem :> Vehicle;

    // `individual` on a usage: OccurrenceUsage::isIndividual (SysML v2 §8.3.9.11).
    individual testSystem : TestSystem;

    // The modifier is orthogonal to the keyword that declares the usage.
    individual part testVehicle : TestSystem;
    individual occurrence testFlight : Flight;

    // `snapshot` on a usage: OccurrenceUsage::portionKind = snapshot.
    snapshot occurrence takeoff : Flight;
    snapshot part vehicleAtTakeoff : Vehicle;
    snapshot atLiftoff : Flight;

    part def System {
        individual testVehicle : TestSystem;
        individual part spareVehicle : TestSystem;
        individual snapshot spareAtTakeoff : Flight;
        snapshot part vehicleAtLanding : Vehicle;

        action collectData {
            in individual subjectVehicle : TestSystem;
            in snapshot vehicleState : Flight;
        }
    }
}
