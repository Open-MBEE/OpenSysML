// Oracle (docs/project/behavior-semantic-oracle.md): entering `work` starts a
// region with one logged entry beside one with two. The single entry falls
// before, between or after the other region's pair, whose own order is fixed.
package RegionEntryOrderUneven {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            state left {
                entry; then l0;
                state l0 { entry { assign log := log + "l0(entry) "; } }
            }
            state right {
                entry; then outerR;
                state outerR {
                    entry { assign log := log + "outerR(entry) "; }
                    entry; then innerR;
                    state innerR { entry { assign log := log + "innerR(entry) "; } }
                }
            }
        }

        transition first idle accept Go then work;
    }
}
