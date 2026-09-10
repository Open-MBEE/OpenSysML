package Test {
    private import ScalarValues::*;
    private import SI::*;

    // A do action body waiting on the clock pauses there while the machine goes
    // on: the timer set on `watching` fires at 10 s while the body is paused at
    // its second accept (due at 33 s), and exiting the state ends the body, so
    // the wait is withdrawn from the clock and the second count never runs.
    state def Machine {
        attribute ticks : Integer = 0;

        entry; then watching;
        state watching {
            do action poll {
                first start;
                then action wait accept after 3 [s];
                then action count assign ticks := ticks + 1;
                then action again accept after 30 [s];
                then action countAgain assign ticks := ticks + 10;
                then done;
            }
        }
        transition first watching accept after 10 [s] then finished;
        state finished;
    }
}
