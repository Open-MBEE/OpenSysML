// An autonomous machine that leaves `outer` from `second` and comes back
// through its shallow history: the recorded trace shows the history re-entering
// `second` rather than the `first` its default transition names.
package Test {
    private import StateMachines::*;
    state HistoryRevisit {
        attribute departures : Integer = 0;

        entry; then start;
        state start;
        state outer {
            state first;
            state second;
            #shallowHistory state resume;
        }
        state away;

        transition first start then outer.first;
        transition first outer.first then outer.second;
        transition first outer.second if departures == 0 do {
            assign departures := departures + 1;
        } then away;
        transition first outer.second if departures > 0 then done;
        transition first away then outer.resume;
        transition first outer.resume then outer.first;
    }
}
