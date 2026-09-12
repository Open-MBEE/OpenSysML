// A shallow history that has no default transition, reached before its
// composite state was ever left: nothing is recorded, so `outer` is entered as
// any transition into it would enter it — through its own entry transition,
// into `first`. Once `outer` has been left from `second`, the same history
// restores `second`.
package Test {
    state HistoryEmptyDefaultEntry {
        attribute departures : Integer = 0;

        entry; then start;
        state start;
        state outer {
            entry; then first;
            state first;
            state second;
            history resume;
        }
        state away;

        transition first start then away;
        transition first away then resume;
        transition first first then second;
        transition first second if departures == 0 do {
            assign departures := departures + 1;
        } then away;
        transition first second if departures > 0 then done;
    }
}
