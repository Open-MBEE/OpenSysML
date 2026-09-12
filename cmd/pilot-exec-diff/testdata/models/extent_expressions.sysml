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

package Collections {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Rim { attribute radius : Real = 2.0; }
    part def Hub;
    part def Spare;
    part def Rod;

    part rims : Rim[2];
    part hubs : Hub[1..*];
    part spares : Spare[0..*];
    part rods : Rod[2] = (new Rod(), new Rod());

    attribute rimCount : Natural = size(all Rim);
    attribute rimsRead : Natural = size(rims);
    attribute firstRimRadius : Real = rims#(1).radius;
    attribute rimRadii : Real[*] nonunique = rims.radius;
    attribute hubCount : Natural = size(all Hub);
    attribute spareCount : Natural = size(all Spare);
    attribute rodCount : Natural = size(rods);
}

package Mismatch {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Rod;
    part uneven : Rod[2] = (new Rod(), new Rod(), new Rod());

    attribute unevenCount : Natural = size(uneven);
}
