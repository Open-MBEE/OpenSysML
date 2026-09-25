package Test {
    attribute def Go;

    state Machine {
        entry; then init;
        state init;
        state waiting;
        // Sourceless transition: leaves the state declared before it, waiting.
        accept Go then active;
        state active;

        succession first init then waiting;
        succession first active then done;
    }
}
