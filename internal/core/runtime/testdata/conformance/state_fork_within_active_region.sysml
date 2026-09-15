package ForkWithinActiveRegion {
    // The fork fires from state `a` of region `left`, into the parallel state
    // `work` declared in the same region: only `a` is exited, the concurrent
    // region `right` keeps `p` active and `outer` is neither exited nor re-entered.
    state Machine {
        attribute log : String = "";

        entry; then outer;
        state outer parallel {
            entry { assign log := log + "outer(entry) "; }
            exit { assign log := log + "outer(exit) "; }
            state left {
                entry; then a;
                state a {
                    exit { assign log := log + "a(exit) "; }
                }
                fork split;
                state work parallel {
                    entry { assign log := log + "work(entry) "; }
                    state one {
                        state x { entry { assign log := log + "x(entry) "; } }
                        transition first x then done;
                    }
                    state two {
                        state y { entry { assign log := log + "y(entry) "; } }
                        transition first y then done;
                    }
                }
                transition first a do { assign log := log + "T1(effect) "; } then split;
                transition first split do { assign log := log + "T1.1(effect) "; } then x;
                transition first split do { assign log := log + "T1.2(effect) "; } then y;
                transition first work then done;
            }
            state right {
                entry; then p;
                state p {
                    entry { assign log := log + "p(entry) "; }
                    exit { assign log := log + "p(exit) "; }
                }
                transition first p accept Go then done;
            }
        }
        transition first outer do { assign log := log + "T2(effect) "; } then done;
    }
    attribute def Go;
}
