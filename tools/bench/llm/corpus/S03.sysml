package Drivetrain {
    part def Vehicle;
    part def Car :> Vehicle;
    part def Truck :> Vehicle;
    part def PickupTruck :> Truck;
    part def ElectricCar :> Car;
}
