package Test {
    // A transition's own body, `then t { … }`, is the transition usage's action
    // body: its statements are the steps of the one firing, so a `terminate`
    // among them ends the steps after it — `n` is left at 1, not 9 — while the
    // transition still completes and the target is entered.
    private import ScalarValues::*;

    state def Machine {
        attribute n : Integer = 0;

        entry; then s;
        state s;
        state t;

        transition first s then t {
            assign n := 1;
            terminate;
            assign n := 9;
        }
    }
}
