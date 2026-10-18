package ParallelRegionCompletion {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";
        entry; then S1;
        // PSSM Entering 004: a region without an initial pseudostate completes on default entry.
        state S1 parallel {
            entry action { assign log := log + "S1(entry);"; }
            exit action { assign log := log + "S1(exit);"; }
            state r {
                state S1_1 { entry action { assign log := log + "S1.1(entry);"; } }
                transition first S1_1 then done;
            }
        }
        transition first S1 then done;
    }
}
