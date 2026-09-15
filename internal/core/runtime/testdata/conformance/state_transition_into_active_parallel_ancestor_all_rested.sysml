// Once a transition from every region into their parallel owner has left each
// region resting at the owner, a triggered transition out of the owner still
// fires: the owner is the active leaf, and it is exited once.
package Test {
    state TransitionIntoActiveParallelAncestorAllRested {
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
                transition first b accept Down do { assign log := log + "down(effect);"; } then outer;
            }
        }
        state later {
            entry { assign log := log + "later(entry);"; }
        }
        transition first outer accept Leave do { assign log := log + "leave(effect);"; } then later;
        transition first later accept Finish then done;
    }
    attribute def Up;
    attribute def Down;
    attribute def Leave;
    attribute def Finish;
}
