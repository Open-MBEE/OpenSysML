// A state's do behavior reads the payload of the transition that entered it,
// `raise.l`, whichever draw the entry front makes: the do step may run before or
// after the substates' entries, and reads the same transition either way.
package StateDoReadsEnteringTransition {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Monitor {
        attribute log : String = "";
        attribute seen : Integer = 0;

        entry; then idle;
        state idle;
        state work parallel {
            do { in level : Integer = raise.l ?? 99; assign seen := level; assign log := log + "did "; }
            state left {
                entry; then l1;
                state l1 { entry { assign log := log + "l1(entry) "; } }
            }
            state right {
                entry; then r1;
                state r1 { entry { assign log := log + "r1(entry) "; } }
            }
        }

        transition raise first idle accept l : Level then work;
    }
}
