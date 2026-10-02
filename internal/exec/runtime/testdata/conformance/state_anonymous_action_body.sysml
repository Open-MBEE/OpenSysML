package Test {
    // Entry, do and exit behaviors written as anonymous inline action bodies.
    // The due do step and nested entry can occur in either order before exit.
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
