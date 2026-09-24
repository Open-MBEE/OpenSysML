package Test {
    // Two orthogonal regions whose inline do bodies loop and branch. The bodies
    // yield after each statement — a `for` body after each iteration, a nested
    // block after each of its statements — so the regions share the machine one
    // statement at a time as state_concurrent_do's one-action-per-statement form
    // does: the region the entry draw entered first enters its work one step
    // before the other, then the two interleave, the order within a round a
    // choice the policy draws (1,2,4,3,5,6 in declaration order). A body run to
    // its end at entry would give 1,2,3,4,5,6 instead.
    private import ScalarValues::*;

    state def Interleave parallel {
        state left {
            entry; then lstart;
            state lstart;
            state lwork {
                do action {
                    for i in 1..2 {
                        assign seq := seq * 10 + i;
                    }
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
                do action {
                    assign seq := seq * 10 + 4;
                    if seq > 0 {
                        assign seq := seq * 10 + 5;
                        assign seq := seq * 10 + 6;
                    }
                }
            }
            state rdone;

            succession first rstart then rwork;
            succession first rwork then rdone;
        }

        attribute seq : Integer = 0;
    }
}
