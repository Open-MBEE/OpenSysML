package ForkWithinActiveAncestor {
    // The fork fires while `outer`, which declares it, is already active: only
    // `idle` is exited, `outer` neither exits nor re-enters, and `work` is
    // entered by the first branch with its regions starting at the branch targets.
    state Machine {
        attribute log : String = "";

        entry; then outer;
        state outer {
            entry { assign log := log + "outer(entry) "; }
            exit { assign log := log + "outer(exit) "; }
            entry; then idle;
            state idle {
                exit { assign log := log + "idle(exit) "; }
            }
            fork split;
            state work parallel {
                entry { assign log := log + "work(entry) "; }
                state left {
                    state a { entry { assign log := log + "a(entry) "; } }
                    transition first a then done;
                }
                state right {
                    state b { entry { assign log := log + "b(entry) "; } }
                    transition first b then done;
                }
            }
            transition first idle do { assign log := log + "T1(effect) "; } then split;
            transition first split do { assign log := log + "T1.1(effect) "; } then a;
            transition first split do { assign log := log + "T1.2(effect) "; } then b;
            transition first work then done;
        }
        transition first outer do { assign log := log + "T2(effect) "; } then done;
    }
}
