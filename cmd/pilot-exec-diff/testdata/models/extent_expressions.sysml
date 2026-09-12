package Extent {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    enum def Size { small; medium; large; }

    variation part def Gearbox {
        variant part manual;
        variant part automatic;
    }

    part def Wheel;
    part def Car { part wheels : Wheel[2]; }
    part def Boat;
    part car : Car;

    attribute sizes : Size[*] = all Size;
    ref part wheels : Wheel[*] = all Wheel;
    ref part cars : Car[*] = all Car;
    ref part boats : Boat[*] = all Boat;
    attribute gearboxCount : Natural = size(all Gearbox);
    attribute boatCount : Natural = size(all Boat);
}
