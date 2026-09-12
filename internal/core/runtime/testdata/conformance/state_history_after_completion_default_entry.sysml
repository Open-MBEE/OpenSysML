// A composite left by its own completion transition was left at `done`, and a
// history of a completed configuration is empty: the next entry through the
// history is a default entry, into `first`, not a re-entry of `done` that would
// complete `outer` again. Left from `second` instead, the history restores
// `second`.
package Test {
    state HistoryAfterCompletionDefaultEntry {
        attribute completions : Integer = 0;

        entry; then start;
        state start;
        state outer {
            entry; then first;
            state first;
            state second;
            history resume;
            transition first first when Finish then done;
        }
        state away;

        transition first start when Go then outer;
        transition first outer do { assign completions := completions + 1; } then away;
        transition first away when Back then resume;
        transition first first when Next then second;
        transition first second when Leave then away;
        transition first second when Stop then done;
    }
}
