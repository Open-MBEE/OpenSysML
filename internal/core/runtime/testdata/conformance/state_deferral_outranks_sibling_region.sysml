// A deferral outranks a transition in a sibling region: while `waiting` defers
// Ping, the `right` region's transition on Ping waits too, and fires only once
// `waiting` has left on Go and released the event.
package Test {
    state def SiblingDeferral parallel {
        state left {
            entry; then waiting;
            state waiting {
                defer Ping;
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
