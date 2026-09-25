// The fork fires from `a`, active inside the parallel state `inner` in `work`'s
// region `left`, back into `work`'s own regions: `work` stays active while
// `inner` and its substates exit with `b`, and the branches enter `c` and `d`.
package ForkFromWithinNestedOwnerRegions {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then work;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            exit { assign log := log + "work(exit) "; }
            state left {
                entry; then inner;
                state inner parallel {
                    entry { assign log := log + "inner(entry) "; }
                    exit { assign log := log + "inner(exit) "; }
                    state p {
                        entry; then a;
                        state a {
                            entry { assign log := log + "a(entry) "; }
                            exit { assign log := log + "a(exit) "; }
                        }
                    }
                    state q {
                        entry; then a2;
                        state a2 {
                            entry { assign log := log + "a2(entry) "; }
                            exit { assign log := log + "a2(exit) "; }
                        }
                    }
                }
                state c { entry { assign log := log + "c(entry) "; } }
                transition first c then done;
            }
            state right {
                entry; then b;
                state b {
                    entry { assign log := log + "b(entry) "; }
                    exit { assign log := log + "b(exit) "; }
                }
                state d { entry { assign log := log + "d(entry) "; } }
                transition first d then done;
            }
        }
        fork split;

        transition first a accept Go do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then c;
        transition first split do { assign log := log + "T1.2(effect) "; } then d;
        transition first work do { assign log := log + "T2(effect) "; } then done;
    }
    attribute def Go;
}
