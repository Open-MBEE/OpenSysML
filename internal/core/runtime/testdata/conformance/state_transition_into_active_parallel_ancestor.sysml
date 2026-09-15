// A transition from a state in one region of a parallel state into that
// parallel state leaves its region without re-entering the parallel state,
// which stays active with its other region; the parallel state completes once
// the other region reaches `done`, so its completion transition fires then.
package Test {
    state TransitionIntoActiveParallelAncestor {
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
                transition first b accept Finish then done;
            }
        }
        transition first outer do { assign log := log + "completion(effect);"; } then done;
    }
    attribute def Up;
    attribute def Finish;
}
