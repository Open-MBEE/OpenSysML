// A deep history with no default transition takes the same path as a shallow
// one when nothing is recorded: `outer` is entered through its entry transition
// and `middle` through its own, down to `inner`. After `outer` was left from
// `innerTwo`, the history restores `innerTwo` itself.
package Test {
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
            deep history resume;
        }
        state away;

        transition first start when Go then away;
        transition first away when Back then resume;
        transition first inner when Next then innerTwo;
        transition first innerTwo when Leave then away;
        transition first innerTwo when Finish then done;
    }
}
