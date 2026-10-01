// Two leaving transitions share a name at different depths: the nearer one is
// read bare, the outer one by qualified name, each binding only when it fires.
package StateExitShadowedTransitionName {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    attribute def Step :> Integer;
    attribute def Stop :> Integer;

    state Machine {
        attribute seen : Integer = 0;
        attribute exits : Integer = 0;

        entry; then running;
        state running {
            entry; then fast;
            state fast {
                exit action {
                    in code : Integer[0..1] = T.c ?? Machine::T.c;
                    assign exits := exits + 1;
                    if notEmpty(code) {
                        assign seen := seen * 10 + code;
                    }
                }
            }
            state slow;
            transition T first fast accept c : Step then slow;
            transition first slow then fast;
        }
        state stopped;

        transition T first running accept c : Stop then stopped;
        transition first stopped then done;
    }
}
