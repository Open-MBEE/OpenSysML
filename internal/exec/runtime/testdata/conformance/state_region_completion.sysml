package RegionCompletion {
    state def Machine parallel {
        state left {
            entry; then lstart;
            state lstart;
            transition first lstart when First then done;
        }
        state right {
            entry; then rstart;
            state rstart;
            transition first rstart when Second then done;
        }
    }
}
