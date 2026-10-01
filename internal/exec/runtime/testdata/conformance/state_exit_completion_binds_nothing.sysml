// A transition with no trigger carries no payload: an exit or entry action
// performed by it, or by the machine's own start, reads the transition's payload
// as empty and keeps its parameter's fallback.
package StateExitCompletionBindsNothing {
    private import ScalarValues::*;

    attribute def Go :> Integer;

    state Machine {
        attribute entered : Integer = -1;
        attribute exited : Integer = -1;
        attribute second : Integer = -1;

        entry; then idle;
        state idle {
            entry action {
                in level : Integer = go.l ?? 0;
                assign entered := level;
            }
            exit action {
                in level : Integer = go.l ?? 0;
                assign exited := level;
            }
        }
        state passing {
            exit action {
                in level : Integer = go.l ?? 0;
                assign second := level;
            }
        }

        transition go first idle accept l : Go then passing;
        transition first passing then done;
    }
}
