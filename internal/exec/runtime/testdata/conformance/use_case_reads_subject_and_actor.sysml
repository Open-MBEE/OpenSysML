// The subject and actors of a use case bind as a requirement's do: a step of
// the body reads the objects they are valued with.
package test {
    private import ScalarValues::*;

    part def Drone { attribute mass : Real = 3.5; }
    part def Pilot { attribute name : String = "Ann"; }
    part d : Drone;
    part p : Pilot;

    use case fly {
        subject drone : Drone = d;
        actor pilot : Pilot = p;
        out m;
        out who;

        first start;
        then action weigh { assign m := drone.mass; }
        then action greet { assign who := pilot.name; }
        then done;
    }
}
