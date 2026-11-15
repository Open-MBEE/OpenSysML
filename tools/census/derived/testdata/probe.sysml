package Probe {
    import ScalarValues::*;
    attribute def Temp;
    part def Pump {
        attribute t : Temp;
        ref part other : Pump;
    }
    calc def Area { in r : Real; return a : Real = r * r; }
    action def Run { action step1 : Sub; }
    action def Sub;
    part def Pair { part a : Pump; part b : Pump; connect a to b; }
}
