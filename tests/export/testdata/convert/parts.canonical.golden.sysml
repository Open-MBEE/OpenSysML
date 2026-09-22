package Vehicles {
    part def Vehicle {
        attribute mass : ISQBase::MassValue;
        part engine : Engine[1];
    }
    part def Engine;
}
