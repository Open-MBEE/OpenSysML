// An object whose first machine, `quick`, has ended by the time its second,
// `watch`, runs: a signal sent to the object that `watch`'s state does not
// accept is discarded all the same (UML 2.5.1 §14.2.3.9.1) — the ended machine
// neither takes it nor holds the drop — so the Ping `idle` sends on entry is
// gone by the time the timer moves `watch` to `armed`, whose transition on Ping
// never fires.
package test {
    private import ISQ::*;
    private import SI::*;

    item def Ping;

    part def Panel {
        exhibit state quick {
            entry; then done;
        }
        state watch {
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
