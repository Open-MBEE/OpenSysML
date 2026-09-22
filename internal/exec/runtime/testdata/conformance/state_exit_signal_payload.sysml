// A state's exit action reads the payload of the transition leaving it, `raise.l`,
// which the trigger bound before the source state is exited.
package StateExitSignalPayload {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Monitor {
        attribute exited : Integer = 0;
        attribute effect : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in level : Integer = raise.l;
                assign exited := level;
            }
        }
        state alerting;

        transition raise first idle accept l : Level do assign effect := l then alerting;
        transition first alerting then done;
    }
}
