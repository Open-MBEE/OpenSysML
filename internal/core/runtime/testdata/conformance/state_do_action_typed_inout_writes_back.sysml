package Test {
    private import ScalarValues::*;
    private import SI::*;

    // A do behavior written as a typed action usage binding pins performs the
    // action definition: `n` is bound from `ticks` when the performance begins,
    // the definition's flow waits on the clock, and the inout writes back to
    // `ticks` when the performance ends, at 3 s. The state completes once its do
    // behavior has, which is when the succession out of it is taken.
    state def Machine {
        action def Poll {
            inout n : Integer;
            first start;
            then action wait accept after 3 [s];
            then action count assign n := n + 1;
            then done;
        }

        attribute ticks : Integer = 5;

        entry; then watching;
        state watching {
            do action poll : Poll { inout n = ticks; }
        }
        succession first watching then finished;
        state finished;
    }
}
