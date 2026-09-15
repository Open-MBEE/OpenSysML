// A fork's branches may target states nested below a region's own substates:
// the region is the one the target lies in, transitively, and the branch enters
// every state on the way down to it, each initial state on that way bypassed. A
// region no branch names still starts at its own initial state.
package ForkEntersNestedRegionStates {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            state left {
                entry; then l0;
                state l0 { entry { assign log := log + "l0(entry) "; } }
                state wrapper {
                    entry { assign log := log + "wrapper(entry) "; }
                    entry; then w0;
                    state w0 { entry { assign log := log + "w0(entry) "; } }
                    state a { entry { assign log := log + "a(entry) "; } }
                }
            }
            state right {
                entry; then r0;
                state r0 { entry { assign log := log + "r0(entry) "; } }
                state b { entry { assign log := log + "b(entry) "; } }
            }
            state aux {
                entry; then x0;
                state x0 { entry { assign log := log + "x0(entry) "; } }
            }
        }
        fork split;

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then a;
        transition first split do { assign log := log + "T1.2(effect) "; } then b;
    }
}
