// A transition from inside a parallel state nested in one region of an outer
// parallel state into the outer state completes that region, so the owner's
// history keeps no configuration for it: re-entering the owner through its
// history starts the region at its initial state, not at the nested state the
// transition exited, while the other region restores the state it was left in.
package Test {
    state TransitionIntoActiveParallelGrandparentHistory {
        attribute log : String = "";
        entry; then outer;
        state outer parallel {
            history resume;
            entry { assign log := log + "outer(entry);"; }
            exit { assign log := log + "outer(exit);"; }
            state left {
                entry; then idle;
                state idle {
                    entry { assign log := log + "idle(entry);"; }
                }
                transition first idle accept Go then inner;
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
                transition first b accept Next then d;
                state d {
                    entry { assign log := log + "d(entry);"; }
                    exit { assign log := log + "d(exit);"; }
                }
            }
        }
        state away {
            entry { assign log := log + "away(entry);"; }
        }
        transition first outer accept Leave do { assign log := log + "leave(effect);"; } then away;
        transition first away accept Back do { assign log := log + "back(effect);"; } then resume;
        transition first outer accept Finish then done;
    }
    attribute def Go;
    attribute def Up;
    attribute def Next;
    attribute def Leave;
    attribute def Back;
    attribute def Finish;
}
