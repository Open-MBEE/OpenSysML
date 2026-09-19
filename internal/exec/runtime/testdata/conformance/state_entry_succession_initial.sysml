package test {
    // A succession out of the state machine's own entry subaction names the
    // state it starts in (SysML 7.19.3 `entry; then off;`), so `off` is entered
    // without an `initial` pseudostate being declared.
    state Modes {
        attribute seen : Integer = 0;

        entry; then off;

        state off {
            entry assign seen := seen + 1;
        }

        succession first off then done;
    }
}
