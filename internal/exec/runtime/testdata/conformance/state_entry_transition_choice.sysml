package Test {
    attribute def Go;

    state Machine {
        entry; then idle;
        state idle;
        state working {
            entry action boot { }
            transition boot then route;
            choice route;
            transition route if true then active;
            state active;
        }
        transition first idle accept Go then working;
    }
}
