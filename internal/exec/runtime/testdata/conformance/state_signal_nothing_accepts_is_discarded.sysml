// A signal sent to the machine's object while the machine is settled in a state
// no transition out of which accepts it is discarded, not held in flight for a
// state entered later (UML 2.5.1 §14.2.3.9.1): the Ping `idle` sends on entry
// is gone by the time the timer moves the machine to `armed`, whose transition
// on Ping never fires.
package test {
    private import ISQ::*;
    private import SI::*;

    item def Ping;

    part def Panel {
        state Machine {
            entry; then idle;
            state idle {
                entry action { send new Ping() to self; }
            }
            state armed;
            state done;
            transition first idle accept after 1 [s] then armed;
            transition first armed accept Ping then done;
        }
    }

    part panel : Panel;
}
