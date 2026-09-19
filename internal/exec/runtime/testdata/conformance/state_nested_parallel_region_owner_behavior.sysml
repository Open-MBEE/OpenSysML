package NestedParallelRegionOwner {
    private import ScalarValues::*;

    state def Machine {
        attribute worked : Integer = 0;

        entry; then start;
        state start;
        state outer parallel {
            state a {
                defer Ping;
                do action work {
                    assign worked := worked + 1;
                }
                entry; then a1;
                state a1;
            }
            state b {
                entry; then b1;
                state b1;
            }
        }
        state waited;

        succession first start then outer;
        transition first outer when Leave then waited;
        transition first waited when Ping then done;
    }
}
