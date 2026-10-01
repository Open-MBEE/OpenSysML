package NestedParallelOwnerEntryOnceIntraRegion {
    private import ScalarValues::*;

    state def Machine {
        attribute outerEntered : Integer = 0;
        attribute entered : Integer = 0;
        attribute exited : Integer = 0;

        entry; then outer;
        state outer parallel {
            entry action beginOuter {
                assign outerEntered := outerEntered + 1;
            }
            state left {
                entry action begin {
                    assign entered := entered + 1;
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
        state waited;

        transition first outer when Leave then waited;
    }
}
