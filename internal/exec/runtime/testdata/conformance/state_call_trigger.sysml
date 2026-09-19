package CallTriggerTest {
    // A call trigger fires on an invocation of the operation it names; a trigger
    // declaring a parameter takes only a call carrying that argument, and binds
    // it so the guard can read it.
    state Controller {
        entry; then start;
        state start;
        state idle;
        state braking;
        state cruising;

        succession first start then idle;

        transition first idle accept setSpeed(value) if value > 0 then cruising;
        transition first idle accept brake() then braking;

        transition first cruising then done;
        transition first braking then done;
    }
}
