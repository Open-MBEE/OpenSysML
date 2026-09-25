package Vehicles {
    part def Vehicle {
        attribute mass : ISQ::MassValue;
        part engine : Engine[1];
    }
    part def Engine;
}
