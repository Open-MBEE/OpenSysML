// Oracle: KerML 8.2.3.5 — an endpoint is a name resolved from the namespace it is written in: `idle` inside `right` is right's own, and the machine reaches a nested vertex only by the chain `work.right.idle`.
package test {
    private import ScalarValues::*;
    state def Machine {
        attribute log : Integer = 0;
        entry; then init;
        state init;
        then work.right.idle;
        state work parallel {
            state left {
                entry; then lstart;
                state lstart;
                state idle { entry { assign log := 10 * log + 1; } }
                state busy { entry { assign log := 10 * log + 2; } }
            }
            state right {
                entry; then rstart;
                state rstart;
                state idle { entry { assign log := 10 * log + 3; } }
                state busy { entry { assign log := 10 * log + 4; } }
                transition first idle then busy;
            }
        }
        state finished;
        transition first work.right.busy accept Leave then finished;
    }
    attribute def Leave;
}
