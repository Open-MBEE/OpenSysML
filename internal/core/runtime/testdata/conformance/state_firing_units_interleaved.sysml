// Oracle (docs/project/behavior-semantic-oracle.md): one Go enables a transition
// in each region of `work`. A firing's exit runs before its effect; between the
// two firings nothing is ordered, so `b`'s units fall anywhere among `a`'s: six
// orders. Both targets then complete into a join, whose own order is a separate
// draw and leaves no mark in the log.
package FiringUnitsInterleaved {
    private import ScalarValues::*;

    attribute def Go;

    state Machine {
        attribute log : String = "";

        entry; then work;
        state work parallel {
            state a {
                entry; then a1;
                state a1 { exit { assign log := log + "a1(exit) "; } }
                state a2;
                transition first a1 accept Go do assign log := log + "a(effect) " then a2;
                transition first a2 then sync;
            }
            state b {
                entry; then b1;
                state b1 { exit { assign log := log + "b1(exit) "; } }
                state b2;
                transition first b1 accept Go do assign log := log + "b(effect) " then b2;
                transition first b2 then sync;
            }
        }
        join sync;
        transition first sync then done;
        state done;
    }
}
