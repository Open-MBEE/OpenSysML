// A transition leaving a composite state exits its active substate first; both
// exit actions read the payload of that one transition.
package StateExitNestedReadsOuterTransition {
    private import ScalarValues::*;

    attribute def Stop :> Integer;

    state Machine {
        attribute innerExit : Integer = 0;
        attribute outerExit : Integer = 0;

        entry; then running;
        state running {
            exit action {
                in code : Integer = halt.c;
                assign outerExit := code;
            }
            entry; then fast;
            state fast {
                exit action {
                    in code : Integer = halt.c;
                    assign innerExit := code * 10;
                }
            }
        }
        state stopped;

        transition halt first running accept c : Stop then stopped;
        transition first stopped then done;
    }
}
