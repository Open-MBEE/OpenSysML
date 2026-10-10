package Test {
    attribute def Go;

    state Machine {
        attribute ready : Boolean = false;

        entry; then idle;
        state idle;
        state outer {
            entry action boot { }
            transition boot do action {
                assign ready := true;
            } then pick;
            choice pick;
            transition pick if ready then active;
            state active;
        }
        transition first idle accept Go then outer;
    }
}
