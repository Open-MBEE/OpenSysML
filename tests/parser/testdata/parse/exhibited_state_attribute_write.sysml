package Monitors {
    private import ScalarValues::*;
    item def Abort;
    state def Guard {
        attribute missDistance : Real = 20000.0;
        state cruise;
        state aborting;
        transition first cruise accept when missDistance < 10000.0 then aborting;
    }
    part def Flight {
        exhibit state g : Guard;
        perform action nav {
            first start;
            then action degrade assign g.missDistance := 5000.0;
            then action a send new Abort();
            then done;
        }
    }
}
