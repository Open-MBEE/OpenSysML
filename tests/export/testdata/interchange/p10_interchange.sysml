package P10 {
    private import ScalarValues::*;
    private import ISQ::*;
    part def Vehicle {
        attribute mass : MassValue;
        part wheels : Wheel[4];
        port fuelPort : FuelPort;
    }
    part def Wheel { attribute diameter : Real; }
    port def FuelPort { in item fuel : Fuel; }
    item def Fuel;
    part car : Vehicle {
        attribute :>> mass = 1200 [kg];
        part :>> wheels { attribute :>> diameter = 0.6; }
    }
    requirement def LightVehicle { subject v : Vehicle; require constraint { v.mass < 1500 [kg] } }
    requirement lightCar : LightVehicle { subject v :>> car; }
    satisfy lightCar by car;
}
