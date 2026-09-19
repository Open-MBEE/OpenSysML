// The same machine without the deferral: the Ping arriving while `busy` is
// active is dropped where no transition handles it, so the machine stops in
// `ready`. This is what makes state_deferred_event evidence of deferral rather
// than of queue ordering.
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
