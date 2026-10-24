// A use case stating no flow still binds its subject before its defaults are
// initialized, run on its own or included as a step of another use case.
package test {
    private import ScalarValues::*;

    part def Drone { attribute mass : Real = 3.5; }
    part d : Drone;

    use case def Inspect {
        subject drone : Drone;
        attribute m : Real = drone.mass;
    }

    use case inspect : Inspect {
        subject drone = d;
    }

    use case mission {
        subject drone : Drone = d;
        out total : Real;

        first start;
        then include use case look : Inspect { subject drone = mission::drone; }
        then action sum { assign total := look.m + 1.0; }
        then done;
    }
}
