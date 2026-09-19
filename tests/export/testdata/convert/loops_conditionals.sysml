package LoopsAndConditionals {
    action def Tick;
    action def Control {
        in attribute count;
        attribute total;
        action tick : Tick;
        while count > 0 {
            perform tick;
        }
        loop {
            assign total := total + 1;
        }
        loop {
            perform tick;
        } until total > 100;
        for idx in 1..10 {
            assign total := total + idx;
        }
        if count > 0 {
            perform tick;
        }
        if count > 0 {
            perform tick;
        } else {
            assign total := 0;
        }
        if count > 0 {
            perform tick;
        } else if total > 0 {
            assign total := 0;
        }
    }
}
