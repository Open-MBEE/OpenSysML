package Test {
    private import ScalarValues::*;
    private import SI::*;

    // A do action body written in declaration order, with no `first`, starts at
    // the one node no succession leads to. The body performs once: it waits 3 s
    // on the machine's clock, counts, and ends; the state's do behavior does not
    // run again, so the timed transition out of `watching` at 10 s finds `ticks`
    // at 1 and the machine ends in `finished`.
    state def Machine {
        attribute ticks : Integer = 0;

        entry; then watching;
        state watching {
            do action poll {
                action wait accept after 3 [s];
                then action count assign ticks := ticks + 1;
            }
        }
        transition first watching accept after 10 [s] then finished;
        state finished;
    }
}
