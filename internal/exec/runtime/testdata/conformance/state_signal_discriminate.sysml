package SignalTest {
    // P1 event-matching contract: a state with two signal-triggered transitions
    // must fire the transition whose trigger matches the injected signal.
    // Injecting sigB must reach pathB (finalState = "done"), NOT pathA.
    state Discriminator {
        entry; then start;
        state start;
        state waiting;
        state pathA;
        state pathB;

        succession first start then waiting;

        transition first waiting when sigA then pathA;
        transition first waiting when sigB then pathB;

        transition first pathA then done;
        transition first pathB then done;
    }
}
