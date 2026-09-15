package ForkThroughInactiveAncestors {
    // A machine-level fork enters `outer.work`, two states below the source's level,
    // while `outer` is not active: `outer` is entered on the way without starting
    // `waiting`, the state its entry transition names, and its sibling region
    // `aside` starts as usual; `work` starts at the branch targets.
    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle {
            exit { assign log := log + "idle(exit) "; }
        }
        fork split;
        state outer parallel {
            entry { assign log := log + "outer(entry) "; }
            exit { assign log := log + "outer(exit) "; }
            state main {
                entry; then waiting;
                state waiting {
                    entry { assign log := log + "waiting(entry) "; }
                    exit { assign log := log + "waiting(exit) "; }
                }
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
                transition first work then done;
            }
            state aside {
                entry; then p;
                state p {
                    entry { assign log := log + "p(entry) "; }
                    exit { assign log := log + "p(exit) "; }
                }
                transition first p accept Stop then done;
            }
        }
        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then a;
        transition first split do { assign log := log + "T1.2(effect) "; } then b;
        transition first outer do { assign log := log + "T2(effect) "; } then done;
    }
    attribute def Stop;
}
