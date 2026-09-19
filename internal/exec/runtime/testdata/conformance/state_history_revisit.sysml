// An autonomous machine that leaves `outer` from `second` and comes back
// through its shallow history: the recorded trace shows the history re-entering
// `second` rather than the `first` its default transition names.
package Test {
    state HistoryRevisit {
        attribute departures : Integer = 0;

        entry; then start;
        state start;
        state outer {
            state first;
            state second;
            history resume;
        }
        state away;

        transition first start then first;
        transition first first then second;
        transition first second if departures == 0 do {
            assign departures := departures + 1;
        } then away;
        transition first second if departures > 0 then done;
        transition first away then resume;
        transition first resume then first;
    }
}
