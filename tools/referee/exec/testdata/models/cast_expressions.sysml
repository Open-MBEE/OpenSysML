package Cast {
    private import ScalarValues::*;

    part def Vehicle;
    part def Car :> Vehicle;

    attribute n : Integer = 7;
    attribute r : Real = 2.5;
    attribute whole : Real = 4.0;
    attribute seq : Real[*] = (1, 2.5, 3);
    part car : Car;

    attribute integerAsReal = n as Real;
    attribute integerAsNatural = n as Natural;
    attribute fractionAsInteger = r as Integer;
    attribute wholeAsInteger = whole as Integer;
    attribute sequenceAsInteger = seq as Integer;
    ref part carAsVehicle = car as Vehicle;
    ref part carAsCar = car as Car;
}
