package Test {
    // A transition's braced `do { … }` is one anonymous action: its statements
    // run in order, to the end, at the instant the transition fires. Each digit
    // records one statement; the follow-on transition the next Go triggers runs
    // its own block after the first block is over.
    private import ScalarValues::*;

    attribute def Go;

    state def Machine {
        attribute log : Integer = 0;

        entry; then idle;
        state idle;
        state busy;
        state done;

        transition first idle accept Go do {
            assign log := log * 10 + 1;
            assign log := log * 10 + 2;
            assign log := log * 10 + 3;
        } then busy;
        transition first busy accept Go do {
            assign log := log * 10 + 4;
            assign log := log * 10 + 5;
        } then done;
    }
}
