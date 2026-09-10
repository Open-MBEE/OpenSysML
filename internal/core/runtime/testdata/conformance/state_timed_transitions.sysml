package test {
    // `accept at t` names an instant; `accept after d` an offset from entry.
    // Both timed transitions leave `working`, which is entered at time 10: the
    // `at five` instant is already past and so fires immediately, while
    // `after 3 [s]` would fire at 13. Were `at` read as an offset it would fire
    // at 15 and the machine would sit in `later` instead of reaching `done`.
    private import SI::*;
    private import Time::*;

    state TimedTransitions {
        attribute five : TimeInstantValue = 5 [s];

        entry; then init;
        state init;
        state waiting;
        accept after 10 [s] then working;
        state working;
        accept at five then instant;
        accept after 3 [s] then later;
        state instant;
        accept after 1 [s] then done;
        state later;

        succession first init then waiting;
    }
}
