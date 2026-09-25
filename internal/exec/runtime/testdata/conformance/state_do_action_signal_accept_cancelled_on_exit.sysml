package Test {
    private import ScalarValues::*;
    private import SI::*;

    // A do action body parked at an accept for a signal nothing ever sends is
    // not a deadlock while its state can still be left: the timed transition out
    // of `watching` fires at 10 s, exiting the state ends the body, and the
    // step the accept would have led to never runs.
    attribute def Go;

    state def Machine {
        attribute ticks : Integer = 0;

        entry; then watching;
        state watching {
            do action poll {
                first start;
                then action wait accept g : Go;
                then action count assign ticks := ticks + 1;
                then done;
            }
        }
        transition first watching accept after 10 [s] then finished;
        state finished;
    }
}
