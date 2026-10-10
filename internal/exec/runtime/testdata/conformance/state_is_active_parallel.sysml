package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // States in parallel regions are active together once the parallel state
    // is entered: each region's state reads the parallel state as active from
    // its own entry, and a guard on the parallel state's outgoing transition
    // reads every region's state and the parallel state itself as active at
    // once. The order the regions are entered in is a scheduling choice, so
    // nothing here depends on it.
    item def Go;

    state def Regions {
        attribute parentFromLeft : Boolean = false;
        attribute parentFromRight : Boolean = false;
        attribute allActiveInGuard : Boolean = false;
        attribute finishedFromGuard : Boolean = true;

        entry; then both;
        state both parallel {
            state regionL {
                entry; then left;
                state left {
                    entry assign parentFromLeft := both.isActive;
                }
            }
            state regionR {
                entry; then right;
                state right {
                    entry assign parentFromRight := both.isActive;
                }
            }
        }
        state finished;
        transition first both
            accept Go
            if both.regionL.left.isActive and both.regionR.right.isActive and both.isActive
            do action {
                assign allActiveInGuard := true;
                assign finishedFromGuard := finished.isActive;
            }
            then finished;
    }
}
