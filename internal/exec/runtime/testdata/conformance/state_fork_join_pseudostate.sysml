package ForkJoinTest {
    state ParallelWork {
        attribute leftDone : Integer = 0;
        attribute rightDone : Integer = 0;

        entry; then init;
        state init;
        state ready;
        state working parallel {
            state left {
                entry; then leftStart;
                state leftStart;
                state building { entry { assign leftDone := 1; } }
                succession first leftStart then building;
            }
            state right {
                entry; then rightStart;
                state rightStart;
                state checking { entry { assign rightDone := 1; } }
                succession first rightStart then checking;
            }
        }
        fork split;
        join sync;

        succession first init then ready;
        transition first ready then split;
        transition first split then building;
        transition first split then checking;
        transition first building then sync;
        transition first checking then sync;
        transition first sync then done;
    }
}
