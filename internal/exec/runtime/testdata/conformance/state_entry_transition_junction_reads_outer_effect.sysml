package Test {
    attribute def Go;

    state Machine {
        attribute x : Integer = 0;
        entry; then idle;
        state idle;
        state outer {
            entry action boot { }
            transition boot then pick;
            junction pick;
            transition pick if x > 0 then a;
            transition pick if x <= 0 then b;
            state a;
            state b;
        }
        transition first idle accept Go do assign x := 1 then outer;
    }
}
