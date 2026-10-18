// A use case definition specializing another inherits its body as an action
// definition does: the derived use case runs the base flow, its included use
// case among the steps.
package test {
    private import ScalarValues::*;

    part def Drone;
    part d : Drone;

    use case def Charge {
        subject drone : Drone;
        out charged : Integer;

        first start;
        then action top { assign charged := 10; }
        then done;
    }

    use case def Base {
        subject drone : Drone = d;
        out n : Integer;

        first start;
        then action work { assign n := 1; }
        then include use case charge : Charge;
        then done;
    }

    use case def Derived :> Base;
}
