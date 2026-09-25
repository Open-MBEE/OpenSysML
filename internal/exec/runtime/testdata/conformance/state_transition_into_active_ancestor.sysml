// A transition from a substate into the composite state enclosing it does not
// re-enter the composite, which is already active: the substate is exited, the
// effect runs, and the body the substate left is complete, so the composite's
// completion transition fires. The log pins that `outer` runs neither its
// entry nor its own start again.
package Test {
    state TransitionIntoActiveAncestor {
        attribute log : String = "";
        entry; then wait;
        state wait;
        state outer {
            entry action { assign log := log + "outer(entry);"; }
            exit action { assign log := log + "outer(exit);"; }
            entry; then inner;
            state inner {
                entry action { assign log := log + "inner(entry);"; }
                exit action { assign log := log + "inner(exit);"; }
            }
            transition first inner accept Up do { assign log := log + "up(effect);"; } then outer;
        }

        transition first wait accept Go then outer;
        transition first outer do { assign log := log + "completion(effect);"; } then done;
    }
    attribute def Go;
    attribute def Up;
}
