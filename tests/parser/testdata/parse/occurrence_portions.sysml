// Portions of an occurrence: `snapshot` and `timeslice` prefix a usage that
// portions the occurrence it subsets (SysML v2 8.2.2.8.2, PortionUsage).
package OccurrencePortions {
    item def Vehicle;

    item vehicle : Vehicle {
        attribute mass;
        timeslice item cruise;
        snapshot item takeoff;
    }

    individual def Serial;

    individual serial : Serial {
        timeslice assembled;
        snapshot delivered;
    }

    action def Mission {
        in individual crew : Serial;
    }
}
