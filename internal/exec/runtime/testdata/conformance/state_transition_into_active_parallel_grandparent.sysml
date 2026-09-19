// A transition from a state nested in a parallel state inside one region of an
// outer parallel state into the outer parallel state exits the inner parallel
// state and leaves the outer region resting at the outer state; the outer state
// completes once its other region reaches `done`, so its completion transition
// fires then.
package Test {
    state TransitionIntoActiveParallelGrandparent {
        attribute log : String = "";
        entry; then outer;
        state outer parallel {
            entry { assign log := log + "outer(entry);"; }
            exit { assign log := log + "outer(exit);"; }
            state left {
                entry; then inner;
                state inner parallel {
                    entry { assign log := log + "inner(entry);"; }
                    exit { assign log := log + "inner(exit);"; }
                    state p {
                        entry; then a;
                        state a {
                            entry { assign log := log + "a(entry);"; }
                            exit { assign log := log + "a(exit);"; }
                        }
                        transition first a accept Up do { assign log := log + "up(effect);"; } then outer;
                    }
                    state q {
                        entry; then c;
                        state c {
                            entry { assign log := log + "c(entry);"; }
                            exit { assign log := log + "c(exit);"; }
                        }
                    }
                }
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
