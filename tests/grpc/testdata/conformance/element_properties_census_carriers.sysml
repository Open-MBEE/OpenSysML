// Carriers of the metaclasses the census finds in no other fixture: cases, concerns,
// viewpoints, views, use cases, allocations, flows, renderings, annotations and
// terminations, so the census settles over the fixtures without reading every example.
package CensusCarriers {

    metadata def Tag;
    filter @Tag;

    part def Vehicle;
    part car : Vehicle {
        @Tag;
    }
    part spare : Vehicle;
    comment about car /* the vehicle the carriers below refer to */

    attribute carTags = car.metadata;

    analysis def Budget {
        subject vehicle : Vehicle;
        return remaining;
    }
    analysis budget : Budget {
        subject vehicle = car;
        return remaining = 2.0;
    }

    concern def Safe {
        subject vehicle : Vehicle;
        stakeholder operator : Vehicle;
    }
    concern safe : Safe;
    viewpoint def Operations {
        subject vehicle : Vehicle;
        stakeholder operator : Vehicle;
        frame concern safe : Safe;
    }
    viewpoint operations : Operations;

    rendering def Table;
    rendering table : Table;
    view def Overview {
        satisfy operations;
        filter @Tag;
        render table;
    }
    view overview : Overview {
        expose car;
    }

    use case def Park;
    use case def Drive {
        subject vehicle : Vehicle;
        actor driver : Vehicle;
        include use case park : Park;
    }
    use case drive : Drive {
        subject vehicle = car;
        actor driver : Vehicle;
        include use case park : Park;
    }

    allocation def VehicleToSpare {
        end source : Vehicle;
        end target : Vehicle;
    }
    allocation carToSpare : VehicleToSpare allocate car to spare;

    abstract flow def Message {
        attribute content;
    }

    action def Stop {
        occurrence occ;
        action trm terminate occ;
    }
}
