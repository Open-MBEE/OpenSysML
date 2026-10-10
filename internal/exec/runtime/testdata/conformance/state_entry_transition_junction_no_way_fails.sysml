package Test {
    attribute def Enter;
    attribute def Go;

    state Machine {
        entry; then idle;
        state idle;
        state blocked {
            entry action boot { }
            transition boot then route;
            junction route;
            transition route if false then never;
            state never;
        }
        state recovered;

        transition first idle accept Enter then blocked;
        transition first idle accept Go then recovered;
    }
}
