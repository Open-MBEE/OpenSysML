package Loops {
    action def Charge {
        action prep;
        loop action charging {
            action step;
        } until charging.done;
        while prep.done {
            action pace;
        } until prep.done;
    }
}
