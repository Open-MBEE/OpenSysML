package ParallelEntry {
    private import ScalarValues::*;

    state def Machine parallel {
        attribute entered : Integer = 0;
        entry action begin {
            assign entered := entered + 1;
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
}
