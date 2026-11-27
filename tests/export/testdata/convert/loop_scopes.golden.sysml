package Loops {
    action def Charge {
        action prep;
        loop {
            action charging;
        } until prep.done;
        while prep.done {
            action pace;
        } until prep.done;
        loop action cycle {
            action step;
        } until cycle.done;
    }
}
