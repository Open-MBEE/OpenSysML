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
            junction pick;
            transition pick if ready then active;
            transition pick if not ready then waiting;
            state active;
            state waiting;
        }
        transition first idle accept Go then outer;
    }
}
