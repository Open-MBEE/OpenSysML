package Classify {
    private import ScalarValues::*;

    part def Vehicle;
    part def Car :> Vehicle;

    attribute a : Integer = 3;
    attribute r : Real = 2.5;
    attribute seq : Integer[*] = (1, 2, 3);
    attribute mixed : Real[*] = (1, 2.5, 3);
    part car : Car;

    attribute atInteger = a @ Integer;
    attribute atReal = a @ Real;
    attribute atString = a @ String;
    attribute realAtInteger = r @ Integer;
    attribute seqAtInteger = seq @ Integer;
    attribute mixedAtInteger = mixed @ Integer;
    attribute mixedIsInteger = mixed istype Integer;
    attribute mixedAtReal = mixed @ Real;
    attribute emptyAtInteger = () @ Integer;
    attribute carAtCar = car @ Car;
    attribute carAtVehicle = car @ Vehicle;
    attribute hasInteger = a hastype Integer;
    attribute hasReal = a hastype Real;
    attribute isReal = a istype Real;
}
