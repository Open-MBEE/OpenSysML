// A default the use case definition declares reads the subject and actor the
// usage binds: the bindings are held before the defaults are initialized.
package test {
    private import ScalarValues::*;

    part def Drone { attribute mass : Real = 3.5; }
    part def Pilot { attribute name : String = "Ann"; }
    part d : Drone;
    part p : Pilot;

    use case def Fly {
        subject drone : Drone;
        actor pilot : Pilot;
        attribute startingMass : Real = drone.mass;
        attribute who : String = pilot.name;
        out m : Real;
        out greeting : String;

        first start;
        then action report { assign m := startingMass; assign greeting := who; }
        then done;
    }

    use case fly : Fly {
        subject drone = d;
        actor pilot = p;
    }
}
