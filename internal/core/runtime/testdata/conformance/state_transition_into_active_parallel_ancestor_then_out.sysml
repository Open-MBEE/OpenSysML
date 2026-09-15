// After a transition from one region into its parallel owner left that region
// resting, a triggered transition out of the owner exits the other region's
// active state and the owner once each, and the rested region nothing.
package Test {
    state TransitionIntoActiveParallelAncestorThenOut {
        attribute log : String = "";
        entry; then outer;
        state outer parallel {
            entry { assign log := log + "outer(entry);"; }
            exit { assign log := log + "outer(exit);"; }
            state left {
                entry; then a;
                state a {
                    entry { assign log := log + "a(entry);"; }
                    exit { assign log := log + "a(exit);"; }
                }
                transition first a accept Up do { assign log := log + "up(effect);"; } then outer;
            }
            state right {
                entry; then b;
                state b {
                    entry { assign log := log + "b(entry);"; }
                    exit { assign log := log + "b(exit);"; }
                }
            }
        }
        state after {
            entry { assign log := log + "after(entry);"; }
        }
        transition first outer accept Leave do { assign log := log + "leave(effect);"; } then after;
        transition first after accept Finish then done;
    }
    attribute def Up;
    attribute def Leave;
    attribute def Finish;
}
