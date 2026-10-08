package Branches {
    private import ScalarValues::Integer;
    action def Charge {
        attribute level : Integer;
        action check;
        then decide;
        if level < 100 then addCharge;
        if level >= 100 then stop;
        action addCharge;
        action stop;
        decide pick;
        if level > 1 then addCharge;
        else stop;
    }
}
