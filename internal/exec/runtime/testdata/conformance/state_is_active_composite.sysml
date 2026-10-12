package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // A composite state is active while any of its substates is, so a parent
    // reads true from its child's entry, a sibling of the parent reads false,
    // and a feature chain `outer.inner.isActive` reads the nested state.
    item def Go;

    state def Nested {
        attribute parentFromChild : Boolean = false;
        attribute siblingFromChild : Boolean = true;
        attribute chainFromChild : Boolean = false;
        attribute innerAfterLeaving : Boolean = true;
        attribute outerAfterLeaving : Boolean = true;

        entry; then outer;
        state outer {
            entry; then inner;
            state inner {
                entry action {
                    assign parentFromChild := outer.isActive;
                    assign siblingFromChild := later.isActive;
                    assign chainFromChild := outer.inner.isActive;
                }
            }
        }
        state later {
            entry action {
                assign innerAfterLeaving := outer.inner.isActive;
                assign outerAfterLeaving := outer.isActive;
            }
        }
        transition first outer accept Go then later;
    }
}
