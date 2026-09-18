package Loops {
    action def Charge {
        action prep;
        loop {
            action charging;
        } until charging.done;
        while prep.done {
            action pace;
        } until pace.done;
    }
}
