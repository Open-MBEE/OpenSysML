// A transition out of a composite state into its own deep history restores the
// configuration that very transition leaves, not the one recorded by an earlier
// exit: `outer` left from `innerTwo` on Reset re-enters `innerTwo`, though it
// was first left from `inner`. The log pins that `outer`'s exit and entry
// behaviors run around the effect, with `middle` and `innerTwo` re-entered.
package Test {
    state DeepHistorySelfTransition {
        attribute log : String = "";
        entry; then start;
        state start;
        state outer {
            entry action { assign log := log + "outer(entry);"; }
            exit action { assign log := log + "outer(exit);"; }
            entry; then middle;
            state middle {
                entry action { assign log := log + "middle(entry);"; }
                entry; then inner;
                state inner;
                state innerTwo {
                    entry action { assign log := log + "innerTwo(entry);"; }
                }
                transition first inner when Next then innerTwo;
            }
            deep history resume;
        }
        state away;

        transition first start when Go then outer;
        transition first inner when Leave then away;
        transition first away when Back then resume;
        transition first outer when Reset do { assign log := log + "reset(effect);"; } then resume;
        transition first innerTwo when Finish then done;
    }
}
