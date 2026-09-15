// A fork entered from outside an enclosing composite state completes the
// regions' owner inside it: the owner's completion transition fires and reaches
// the enclosing state's `done`, whose own completion transition then fires.
package ForkCompletesNestedOwner {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state stage {
            entry { assign log := log + "stage(entry) "; }
            exit { assign log := log + "stage(exit) "; }
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
            transition first work do { assign log := log + "T2(effect) "; } then done;
        }

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first stage do { assign log := log + "T3(effect) "; } then done;
    }
}
