// Oracle (docs/project/behavior-semantic-oracle.md): the left region of `work`
// starts in `inner`, itself parallel, so entering `inner` starts two more
// regions beside `right`'s. The three leaf entries and `inner`'s interleave in
// any order that keeps `inner` ahead of the two regions it owns.
package RegionEntryNestedFront {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            state left {
                entry; then inner;
                state inner parallel {
                    entry { assign log := log + "inner(entry) "; }
                    state p {
                        entry; then p0;
                        state p0 { entry { assign log := log + "p0(entry) "; } }
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
            }
        }

        transition first idle accept Go then work;
    }
}
