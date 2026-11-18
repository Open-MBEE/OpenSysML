package ChainedThen {
    state def Machine {
        entry; then init;
        state init;
        then work.right.idle;
        state work parallel {
            state left {
                entry; then idle;
                state idle;
                state busy;
                then left.idle;
            }
            state right {
                entry; then idle;
                state idle;
                state busy;
                transition first idle then busy;
            }
        }
        transition first work.right.busy then done;
    }
}
