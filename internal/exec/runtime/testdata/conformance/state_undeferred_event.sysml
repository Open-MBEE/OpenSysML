// The machine of state_deferred_signal_kept_and_replayed without the buffer:
// the Ping arriving while `busy` is active is dropped where no transition
// handles it, so the machine stops in `ready`. This is what makes that case
// evidence of the kept signal rather than of queue ordering.
package Test {
    state UndeferredEvent {
        entry; then start;
        state start;
        state busy;
        state ready;

        succession first start then busy;

        transition first busy when Go then ready;
        transition first ready when Ping then done;
    }
}
