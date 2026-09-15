// A region a fork leaves out, declared before the regions its branches enter,
// does not bring the composite in ahead of the branches: the first branch's
// effect runs, then the composite and its regions enter in declaration order.
package ForkOmittedRegionDeclaredFirst {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            entry { assign log := log + "work(entry) "; }
            state aux {
                entry; then x0;
                state x0 { entry { assign log := log + "x0(entry) "; } }
            }
            state left {
                state a { entry { assign log := log + "a(entry) "; } }
            }
            state right {
                state b { entry { assign log := log + "b(entry) "; } }
            }
        }
        fork split;

        transition first idle do { assign log := log + "T1(effect) "; } then split;
        transition first split do { assign log := log + "T1.1(effect) "; } then a;
        transition first split do { assign log := log + "T1.2(effect) "; } then b;
    }
}
