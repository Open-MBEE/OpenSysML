package ForkEntersRegionsWithoutInitial {
    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            exit { assign log := log + "work(exit) "; }
            state left {
                state a { entry { assign log := log + "a(entry) "; } exit { assign log := log + "a(exit) "; } }
            }
            state right {
                state b { entry { assign log := log + "b(entry) "; } exit { assign log := log + "b(exit) "; } }
            }
        }
        fork split;
        join sync;

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then a;
        transition first split do { assign log := log + "T1.2(effect) "; } then b;
        transition first a then sync;
        transition first b then sync;
        transition first sync do { assign log := log + "T2(effect) "; } then done;
    }
}
