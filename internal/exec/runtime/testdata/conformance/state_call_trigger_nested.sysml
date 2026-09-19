package CallTriggerNestedTest {
    // The sourceless nested form: a call trigger written inside the state it
    // leaves, so only the invocation carrying `level` takes the lamp out of standby.
    state Lamp {
        entry; then start;
        state start;
        state standby;
        accept turnOn(level) then lit;
        state lit;

        succession first start then standby;
        succession first lit then done;
    }
}
