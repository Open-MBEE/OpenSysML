package CallTriggers {
    state Controller {
        entry; then start;
        state start;
        state idle;
        accept brake() then stopping;
        state stopping;
        state cruising;

        succession first start then idle;

        transition first idle accept setSpeed(value) if value > 0 then cruising;
    }
}
