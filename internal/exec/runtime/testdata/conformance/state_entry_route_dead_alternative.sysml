package test {
    attribute def Go;

    state Machine {
        entry; then idle;
        state idle;
        state outer {
            entry; then route;
            junction route;
            state good;
            state bad {
                entry; then blocked;
                junction blocked;
                transition blocked if false then never;
                state never;
            }
            transition first route then good;
            transition first route then bad;
        }
        transition first idle accept Go then outer;
    }
}
