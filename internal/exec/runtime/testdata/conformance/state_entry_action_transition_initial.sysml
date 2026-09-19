package test {
    private import ScalarValues::*;

    // A transition out of the machine's named entry action names the state it
    // starts in (SysML 7.19.3), the same as the anonymous `entry; then off;`.
    state Modes {
        attribute seen : Integer = 0;

        entry action begin { }
        transition begin then off;

        state off {
            entry assign seen := seen + 1;
        }

        succession first off then done;
    }
}
