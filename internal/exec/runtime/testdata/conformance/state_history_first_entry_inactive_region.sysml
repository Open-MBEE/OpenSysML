package HistoryFirstEntryInactiveRegion {
    attribute def Enter;
    attribute def Go;

    state Machine {
        attribute log : String = "";
        entry; then idle;
        state idle;
        state P parallel {
            history resume;
            state a {
                entry; then A1;
                state A1 { entry { assign log := log + "A1(entry);"; } }
                transition first A1 accept Go then done;
            }
            state b {
                state B1 { entry { assign log := log + "B1(entry);"; } }
            }
        }
        state finished { entry { assign log := log + "finished(entry);"; } }
        transition first idle accept Enter then P::resume;
        transition first P then finished;
    }
}
