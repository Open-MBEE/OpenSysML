// A state's exit action reads the arguments of the operation call whose trigger
// fires the leaving transition, each by the trigger's parameter name.
package StateExitCallArguments {
    private import ScalarValues::*;

    state Controller {
        attribute exitedSpeed : Integer = 0;
        attribute exitedGear : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in v : Integer = set.speed;
                in g : Integer = set.gear;
                assign exitedSpeed := v;
                assign exitedGear := g;
            }
        }
        state cruising;

        transition set first idle accept setSpeed(speed, gear) then cruising;
        transition first cruising then done;
    }
}
