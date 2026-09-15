// A fork's branch may target a state inside a parallel state nested in the
// region it enters: the nested state's region the target lies in starts there,
// its initial state bypassed, and its other regions start as usual.
package ForkEntersNestedParallelState {
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
                state inner parallel {
                    entry { assign log := log + "inner(entry) "; }
                    state p {
                        entry; then p0;
                        state p0 { entry { assign log := log + "p0(entry) "; } }
                        state a { entry { assign log := log + "a(entry) "; } }
                    }
                    state q {
                        entry; then q0;
                        state q0 { entry { assign log := log + "q0(entry) "; } }
                    }
                }
            }
            state right {
                entry; then r0;
                state r0 { entry { assign log := log + "r0(entry) "; } }
                state b { entry { assign log := log + "b(entry) "; } }
            }
        }
        fork split;

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then a;
        transition first split do { assign log := log + "T1.2(effect) "; } then b;
    }
}
