// A transition into a parallel state whose every region starts in `done`
// completes the machine, running the machine's own exit behavior once.
package test {
    private import ScalarValues::*;
    attribute def Go;

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
    }
}
