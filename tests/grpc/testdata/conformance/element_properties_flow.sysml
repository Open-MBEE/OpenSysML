// The Flow metaclass family read by specification name: an item flow between
// two ports, its payload and the ends it connects.
package FlowFamily {
    private import ScalarValues::*;
    item def Fuel;
    port def FuelPort { out item fuel : Fuel; }
    port def TankPort { in item fuel : Fuel; }
    part def Pump { port supply : FuelPort; }
    part def Tank { port intake : TankPort; }
    part def Vehicle {
        part pump : Pump;
        part tank : Tank;
        flow of Fuel from pump.supply.fuel to tank.intake.fuel;
    }
}
