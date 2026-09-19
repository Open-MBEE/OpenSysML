package NestedParallelBehavior {
    private import ScalarValues::*;

    state def Machine {
        attribute entered : Integer = 0;
        attribute exited : Integer = 0;

        entry; then start;
        state start;
        state outer parallel {
            entry action begin {
                assign entered := entered + 1;
            }
            exit action finish {
                assign exited := exited + 1;
            }
            state left {
                entry; then lidle;
                state lidle;
            }
            state right {
                entry; then ridle;
                state ridle;
            }
        }
        state done;

        succession first start then outer;
        transition first outer when Finish then done;
    }
}
