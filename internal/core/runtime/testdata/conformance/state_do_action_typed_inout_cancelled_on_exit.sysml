package Test {
    private import ScalarValues::*;
    private import SI::*;

    // Exiting a state ends a typed do performance before it finishes, and an
    // inout pin of a performance that never ends is not written back: `n` was
    // counted inside the performance at 3 s, but the machine leaves `watching` at
    // 10 s while the performance is paused until 33 s, so `ticks` keeps its 5.
    state def Machine {
        action def Poll {
            inout n : Integer;
            first start;
            then action wait accept after 3 [s];
            then action count assign n := n + 1;
            then action again accept after 30 [s];
            then done;
        }

        attribute ticks : Integer = 5;

        entry; then watching;
        state watching {
            do action poll : Poll { inout n = ticks; }
        }
        transition first watching accept after 10 [s] then finished;
        state finished;
    }
}
