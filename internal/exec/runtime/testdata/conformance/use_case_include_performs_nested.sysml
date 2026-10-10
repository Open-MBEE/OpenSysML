// An `include use case` member performs the included use case as a nested
// action: its body runs in the step, its parameters bind from the including
// use case's, and a later step reads what it produced.
package test {
    part def Drone;
    part d : Drone;

    use case def Charge {
        subject drone : Drone;
        in level;
        out charged;

        first start;
        then action plug;
        then action top { assign charged := level + 10; }
        then done;
    }

    use case fly {
        subject drone : Drone = d;
        in x default 1;
        out y;

        first start;
        then include use case charge : Charge { in level = fly::x; }
        then action go { assign y := charge.charged + 1; }
        then done;
    }
}
