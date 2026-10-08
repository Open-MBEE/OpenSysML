package Test {
    attribute def Go;

    state Machine {
        attribute log : Integer = 0;
        entry; then idle;
        state idle;
        state working {
            entry action boot { }
            transition boot if false do action {
                assign log := 9;
            } then rejected;
            transition boot then accepted;
            state rejected;
            state accepted;
        }
        transition first idle accept Go then working;
    }
}
