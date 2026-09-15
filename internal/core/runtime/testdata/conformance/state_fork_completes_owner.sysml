// A fork whose branches all end in `done` completes the composite state that
// owns the regions as the branches enter: its completion transition fires
// without any further occurrence, and the machine completes through its `done`.
package ForkCompletesOwner {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            exit { assign log := log + "work(exit) "; }
            state left {
                fork split;
                transition first split do { assign log := log + "T1.1(effect) "; } then done;
            }
            state right {
                transition first split do { assign log := log + "T1.2(effect) "; } then done;
            }
        }

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first work do { assign log := log + "T2(effect) "; } then done;
    }
}
