// Oracle (docs/project/behavior-semantic-oracle.md): a fork's two branches enter
// the regions of `top`, which neither has entered yet. Each branch runs its
// effect, then enters `top` unless the other already has, then its target; the
// two branches interleave, and `top` is entered exactly once, after the first
// branch effect and never before both.
package ForkBranchOrder {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state top parallel {
            entry { assign log := log + "top(entry) "; }
            state left {
                state a;
            }
            state right {
                state b;
            }
        }
        fork split;

        transition T1 first idle do { assign log := log + "T1(effect) "; } then split;
        transition T1a first split do { assign log := log + "T1a(effect) "; } then a;
        transition T1b first split do { assign log := log + "T1b(effect) "; } then b;
    }
}
