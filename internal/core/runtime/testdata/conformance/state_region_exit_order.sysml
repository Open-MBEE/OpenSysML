// Oracle (docs/project/behavior-semantic-oracle.md): leaving `work` exits its two
// regions together. Within `left` the library orders the inner state's exit
// before the outer one's; across the regions it orders nothing, so `right`'s one
// exit falls before, between or after `left`'s pair, and `work`'s own exit last.
package RegionExitOrder {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then work;
        state work parallel {
            exit { assign log := log + "work(exit) "; }
            state left {
                entry; then outerL;
                state outerL {
                    exit { assign log := log + "outerL(exit) "; }
                    entry; then innerL;
                    state innerL { exit { assign log := log + "innerL(exit) "; } }
                }
            }
            state right {
                entry; then r;
                state r { exit { assign log := log + "r(exit) "; } }
            }
        }
        state done;

        transition first work accept Stop then done;
    }
}
