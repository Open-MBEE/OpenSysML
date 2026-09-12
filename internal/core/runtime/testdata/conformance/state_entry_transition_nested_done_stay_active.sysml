// A transition into a parallel state whose every region starts in `done`
// completes that state on entry, but with no completion transition out of it
// the machine keeps running: the machine's exit behavior does not run, and the
// completed state still takes the event it accepts.
package test {
    private import ScalarValues::*;
    attribute def Go;
    attribute def Again;

    state sm {
        attribute log : String = "";

        entry action { assign log := log + "in;"; }
        then idle;
        exit action { assign log := log + "out;"; }
        state idle;
        transition first idle accept Go then outer;
        state outer parallel {
            entry action { assign log := log + "outer;"; }
            state left {
                entry; then done;
            }
            state right {
                entry; then done;
            }
        }
        transition first outer accept Again then idle;
    }
}
