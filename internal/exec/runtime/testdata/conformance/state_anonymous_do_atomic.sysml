package Test {
    // An anonymous do body written as one action yields after each statement, as
    // state_concurrent_do's one-action-per-statement `do { … }` form does, so the
    // two regions interleave inside it statement by statement: 124356 in entry
    // order, the same digits in the same set of orders as that form. The body is
    // not one atomic action of a round.
    state def AtomicDo parallel {
        attribute seq : Integer = 0;

        state left {
            entry; then lstart;
            state lstart;
            state lwork {
                do action {
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
                do action {
                    assign seq := seq * 10 + 4;
                    assign seq := seq * 10 + 5;
                    assign seq := seq * 10 + 6;
                }
            }
            state rdone;

            succession first rstart then rwork;
            succession first rwork then rdone;
        }
    }
}
