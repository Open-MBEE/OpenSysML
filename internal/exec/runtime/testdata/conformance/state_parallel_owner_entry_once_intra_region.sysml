package ParallelOwnerEntryOnceIntraRegion {
    private import ScalarValues::*;

    state def Machine parallel {
        attribute entered : Integer = 0;
        attribute exited : Integer = 0;
        attribute worked : Integer = 0;
        state left {
            entry action begin {
                assign entered := entered + 1;
            }
            do action work {
                assign worked := worked + 1;
            }
            exit action finish {
                assign exited := exited + 1;
            }
            entry; then prep;
            state prep;
            state work;
            transition first prep when Go then work;
        }
        state right {
            entry; then ridle;
            state ridle;
        }
    }
}
