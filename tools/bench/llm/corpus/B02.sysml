package Lamp {
    state def LampStates {
        entry; then off;
        state off;
        state on;
        transition off_to_on first off then on;
        transition on_to_off first on then off;
    }
}
