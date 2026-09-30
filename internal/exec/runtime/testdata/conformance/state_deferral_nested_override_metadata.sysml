// A transition nested in the deferring state overrides its deferral: `outer`
// defers Continue, but its substate `inner` has a transition on Continue, which
// is nested more deeply than the deferral and so fires and consumes the event.
package Test {
    private import StateMachines::*;
    item def Continue;
    state NestedOverride {
        entry; then outer;
        state outer {
            #deferred ref : Continue;
            entry; then inner;
            state inner;
            state seen;
            transition first inner when Continue then seen;
        }

        transition first outer when Finish then done;
    }
}
