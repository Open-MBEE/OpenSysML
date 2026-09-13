// A deferral outranks a transition out of a state enclosing the deferring one:
// `outer` has a transition on Ping, but while its substate `waiting` defers Ping
// the event is held back, and `outer` leaves on it only once `waiting` has been
// exited and the Ping released.
package Test {
    state EnclosingDeferral {
        entry; then outer;
        state outer {
            entry; then waiting;
            state waiting {
                defer Ping;
            }
            state open;
            transition first waiting when Go then open;
        }
        state pinged;

        transition first outer when Ping then pinged;
    }
}
