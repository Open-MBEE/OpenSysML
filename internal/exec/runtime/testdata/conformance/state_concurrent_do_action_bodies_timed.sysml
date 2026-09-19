package Test {
    private import ScalarValues::*;
    private import SI::*;

    // Two orthogonal regions whose do behaviors are action bodies waiting on the
    // clock. Each body pauses at its accept and is resumed in the round its wait
    // ends in; when both are due at one instant their order is a choice the
    // policy draws, entry order by default: at 2 s left counts 1 and right 3,
    // at 4 s left counts 2, at 5 s right counts 4, so `seq` reads 1324. Each
    // region completes once its do behavior has, which is when its succession
    // out of the working state is taken.
    state def Interleave parallel {
        state left {
            entry; then lwork;
            state lwork {
                do action lpoll {
                    first start;
                    then action w1 accept after 2 [s];
                    then action c1 assign seq := seq * 10 + 1;
                    then action w2 accept after 2 [s];
                    then action c2 assign seq := seq * 10 + 2;
                    then done;
                }
            }
            state ldone;
            succession first lwork then ldone;
        }

        state right {
            entry; then rwork;
            state rwork {
                do action rpoll {
                    first start;
                    then action w3 accept after 2 [s];
                    then action c3 assign seq := seq * 10 + 3;
                    then action w4 accept after 3 [s];
                    then action c4 assign seq := seq * 10 + 4;
                    then done;
                }
            }
            state rdone;
            succession first rwork then rdone;
        }

        attribute seq : Integer = 0;
    }
}
