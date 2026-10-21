package LibraryUser {
    private import ISQ::*;
    private import ScalarValues::*;

    part def Vehicle {
        attribute mass : ISQ::mass;
        attribute wheelCount : ScalarValues::Integer;
        attribute speedFactor : ScalarValues::Real;
        attribute name : ScalarValues::String;
    }

    part vehicle : Vehicle {
        attribute redefines wheelCount = 4;
        attribute redefines speedFactor = 1.5;
    }
}
