// A deferring state nested deeper than a sibling region's transition still
// outranks it: `deep`, inside `left`'s `waiting`, defers Ping, so `right`'s
// transition on Ping waits until `waiting` is left on Go.
package Test {
    state def NestedSiblingDeferral parallel {
        state left {
            entry; then waiting;
            state waiting {
                entry; then deep;
                state deep {
                    defer Ping;
                }
            }
            state open;
            transition first waiting when Go then open;
        }

        state right {
            entry; then idle;
            state idle;
            state pinged;
            transition first idle when Ping then pinged;
        }
    }
}
