package CallTriggerGuardTest {
    // A guard reading an argument the call trigger bound: the invocation matches
    // the trigger but the guard rejects it, so the machine stays in idle.
    state Controller {
        entry; then start;
        state start;
        state idle;
        state cruising;

        succession first start then idle;

        transition first idle accept setSpeed(value) if value > 0 then cruising;
    }
}
