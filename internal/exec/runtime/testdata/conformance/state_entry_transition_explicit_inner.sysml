package Test {
    attribute def Enter;

    state Machine {
        entry; then outside;
        state outside;
        state A {
            entry; then startRoute;
            junction startRoute;
            transition startRoute if false then never;
            state never;
            state inner;
        }

        transition first outside accept Enter then A.inner;
    }
}
