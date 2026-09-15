// The fork fires from state `a` of region `left` back into `work`'s own regions:
// `work` stays active, every active state its regions hold exits — `a` and `b`,
// in region order, `b`'s do behavior with it — and the branches enter `c` and `d`.
package ForkFromWithinOwnerRegions {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then work;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            exit { assign log := log + "work(exit) "; }
            state left {
                entry; then a;
                state a {
                    entry { assign log := log + "a(entry) "; }
                    exit { assign log := log + "a(exit) "; }
                }
                state c { entry { assign log := log + "c(entry) "; } }
                transition first c then done;
            }
            state right {
                entry; then b;
                state b {
                    entry { assign log := log + "b(entry) "; }
                    do action poll {
                        first start;
                        then action wait accept n : Never;
                        then action note assign log := log + "b(do) ";
                        then done;
                    }
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
    attribute def Never;
}
