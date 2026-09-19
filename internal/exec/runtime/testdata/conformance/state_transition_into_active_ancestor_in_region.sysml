// A substate transitioning into the composite enclosing it, both inside an
// orthogonal region: the composite is not re-entered, its body completes and
// its completion transition fires while the sibling region stays as it was.
package Test {
    state TransitionIntoActiveAncestorInRegion {
        attribute log : String = "";
        entry; then outer;
        state outer parallel {
            state left {
                entry; then mid;
                state mid {
                    entry { assign log := log + "mid(entry);"; }
                    exit { assign log := log + "mid(exit);"; }
                    entry; then inner;
                    state inner {
                        entry { assign log := log + "inner(entry);"; }
                        exit { assign log := log + "inner(exit);"; }
                    }
                    transition first inner accept Up do { assign log := log + "up(effect);"; } then mid;
                }
                state leftDone {
                    entry { assign log := log + "leftDone(entry);"; }
                }
                transition first mid do { assign log := log + "completion(effect);"; } then leftDone;
            }
            state right {
                entry; then b;
                state b {
                    entry { assign log := log + "b(entry);"; }
                    exit { assign log := log + "b(exit);"; }
                }
            }
        }
        transition first outer accept Leave then done;
    }
    attribute def Up;
    attribute def Leave;
}
