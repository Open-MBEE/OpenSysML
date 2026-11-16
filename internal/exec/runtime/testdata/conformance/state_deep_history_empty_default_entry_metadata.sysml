// A deep history with no default transition takes the same path as a shallow
// one when nothing is recorded: `outer` is entered through its entry transition
// and `middle` through its own, down to `inner`. After `outer` was left from
// `innerTwo`, the history restores `innerTwo` itself.
package Test {
    private import StateMachines::*;
    state DeepHistoryEmptyDefaultEntry {
        entry; then start;
        state start;
        state outer {
            entry; then middle;
            state middle {
                entry; then inner;
                state inner;
                state innerTwo;
            }
            #deepHistory state resume;
        }
        state away;

        transition first start when Go then away;
        transition first away when Back then outer.resume;
        transition first outer.middle.inner when Next then outer.middle.innerTwo;
        transition first outer.middle.innerTwo when Leave then away;
        transition first outer.middle.innerTwo when Finish then done;
    }
}
