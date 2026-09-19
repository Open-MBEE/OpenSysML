// A history of a completed configuration is empty, and an empty history with a
// default transition takes it: `outer` left from `first` is restored to `first`;
// left at `done`, it is re-entered through the history's own `then second`, not
// through `outer`'s entry into `first`.
package Test {
    state HistoryAfterCompletionDefaultTransition {
        entry; then start;
        state start;
        state outer {
            entry; then first;
            state first;
            state second;
            history resume;
            transition first resume then second;
            transition first first when Finish then done;
            transition first second when Stop then done;
        }
        state away;

        transition first start when Go then outer;
        transition first outer then away;
        transition first away when Back then resume;
        transition first first when Leave then away;
    }
}
