package Test {
    attribute def Go;

    state Machine {
        attribute log : Integer = 0;

        entry; then idle;
        state idle;
        state working {
            entry action boot { assign log := log * 10 + 1; }
            transition boot do action {
                assign log := log * 10 + 2;
            } then active;
            state active {
                entry action { assign log := log * 10 + 3; }
            }
        }
        transition first idle accept Go then working;
    }
}
