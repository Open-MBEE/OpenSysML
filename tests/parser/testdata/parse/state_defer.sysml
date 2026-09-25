package Deferral {
    state Controller {
        entry; then start;
        state start;
        state busy {
            defer Ping, setSpeed(value);
        }
        state ready;

        succession first start then busy;

        transition first busy when Go then ready;
        transition first ready when Ping then done;
    }
}
