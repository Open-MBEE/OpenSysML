// Oracle (docs/project/behavior-semantic-oracle.md): entering `work` starts its
// two regions together. Within each region the library orders the outer state's
// entry before the inner one's; across the regions it orders nothing, so the
// four entries interleave in any order keeping each region's two in sequence.
package RegionEntryOrder {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state work parallel {
            state left {
                entry; then outerL;
                state outerL {
                    entry { assign log := log + "outerL(entry) "; }
                    entry; then innerL;
                    state innerL { entry { assign log := log + "innerL(entry) "; } }
                }
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
