package Test {
    // Two orthogonal regions whose do behaviors run while their states are
    // active. Each digit records one do action, so the resulting number shows
    // the interleaving: the region whose start the entry draw entered first
    // completes first and enters its work one step before the other, then the
    // two share the machine one action at a time, the order within a round a
    // choice the policy draws (1,2,4,3,5,6 in declaration order). A do behavior
    // run to its end at entry would give 1,2,3,4,5,6 instead.
    state def Interleave parallel {
        state left {
            entry; then lstart;
            state lstart;
            state lwork {
                do {
                    assign seq := seq * 10 + 1;
                    assign seq := seq * 10 + 2;
                    assign seq := seq * 10 + 3;
                }
            }
            state ldone;

            succession first lstart then lwork;
            succession first lwork then ldone;
        }

        state right {
            entry; then rstart;
            state rstart;
            state rwork {
                do {
                    assign seq := seq * 10 + 4;
                    assign seq := seq * 10 + 5;
                    assign seq := seq * 10 + 6;
                }
            }
            state rdone;

            succession first rstart then rwork;
            succession first rwork then rdone;
        }

        attribute seq : Integer = 0;
    }
}
