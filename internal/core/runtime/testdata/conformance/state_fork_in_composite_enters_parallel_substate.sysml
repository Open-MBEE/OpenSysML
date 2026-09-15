package ForkInCompositeEntersParallelSubstate {
    // The fork's own parent `outer` is entered before its branches fire; the
    // parallel state `work` below it is entered by the first branch, after that
    // branch's effect, and the regions start at the branch targets (no `entry`).
    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state outer {
            entry { assign log := log + "outer(entry) "; }
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
            transition first split do { assign log := log + "T1.1(effect) "; } then a;
            transition first split do { assign log := log + "T1.2(effect) "; } then b;
            transition first work then done;
        }

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first outer do { assign log := log + "T2(effect) "; } then done;
    }
}
