package Test {
    // Entry, do and exit behaviors written as anonymous inline action bodies.
    // Each digit records one statement, so the number shows the ordering: entry
    // loop (3, 2, 1), the do body (8), the nested entry (7) that the region's
    // initial transition reaches one step later, then the exit body (9) once
    // the region has reached `done` as well as the do body having ended.
    state def Bodies {
        attribute log : Integer = 0;

        entry; then start;
        state start;

        state working parallel {
            entry action {
                attribute k : Integer = 3;
                while k > 0 {
                    assign log := log * 10 + k;
                    assign k := k - 1;
                }
            }
            do action { assign log := log * 10 + 8; }
            exit action { assign log := log * 10 + 9; }

            state inside {
                entry; then nstart;
                state nstart;
                state nested {
                    entry action { assign log := log * 10 + 7; }
                    do action { }
                    exit action { }
                }

                succession first nstart then nested;
                succession first nested then done;
            }
        }

        state finished;

        succession first start then working;
        succession first working then finished;
    }
}
