// A state defers an event it cannot handle: the Ping arriving while `busy` is
// active is retained rather than dropped, and is delivered once the machine has
// left `busy` for a state that handles it.
package Test {
    private import StateMachines::*;
    item def Ping;
    state DeferredEvent {
        entry; then start;
        state start;
        state busy {
            #deferred ref : Ping;
        }
        state ready;

        succession first start then busy;

        transition first busy when Go then ready;
        transition first ready when Ping then done;
    }
}
