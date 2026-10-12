package Test {
    attribute def Reset;

    state Machine {
        attribute log : Integer = 0;

        entry; then start;
        state start;
        state outer {
            entry action boot { assign log := log * 10 + 1; }
            transition boot do action {
                assign log := log * 10 + 2;
            } then inner;
            state inner {
                entry action { assign log := log * 10 + 3; }
            }
            deep history resume;
            transition first outer accept Reset then resume;
        }
        transition first start then outer;
    }
}
