package SignalTest {
    // P1 event-matching contract: an injected signal with no matching outgoing
    // transition must be dropped without firing anything.
    // Injecting sigC (no transition) must leave the machine in `waiting`.
    state Unmatched {
        entry; then start;
        state start;
        state waiting;
        state pathA;

        succession first start then waiting;

        transition first waiting when sigA then pathA;
    }
}
