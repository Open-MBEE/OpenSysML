// A deep history declared in the machine's own body restores the machine's
// top-level configuration, which is the one the transition into it leaves:
// Again out of `inner` exits `outer` and re-enters it at `inner`, not at
// `outer`'s initial state `first`. A shallow history in the same body restores
// `outer` and lets it start afresh.
package Test {
    state MachineBodyDeepHistory {
        attribute log : String = "";
        entry; then start;
        state start;
        deep history resume;
        shallow history back;
        state outer {
            entry action { assign log := log + "outer(entry);"; }
            exit action { assign log := log + "outer(exit);"; }
            entry; then first;
            state first;
            state inner {
                entry action { assign log := log + "inner(entry);"; }
            }
            transition first first when Next then inner;
        }

        transition first start when Go then outer;
        transition first inner when Again do { assign log := log + "again(effect);"; } then resume;
        transition first inner when Shallow do { assign log := log + "shallow(effect);"; } then back;
        transition first inner when Finish then done;
    }
}
