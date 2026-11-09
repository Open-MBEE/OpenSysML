// An action declared as a node reports the kind of that node, and a succession
// flow its own kind, rather than the plain action or flow usage of the keyword.
package P {
    private import ScalarValues::*;
    attribute def Sig;
    item def Fuel;
    port def SigPort;
    part def Tank { out item fuelOut : Fuel; }
    part def Engine { in item fuelIn : Fuel; }
    action def Notify;
    action def A {
        in sig : Sig;
        in p : SigPort;
        attribute count : Integer;
        action sendIt send sig via p;
        action acceptIt accept s : Sig via p;
        action assignIt assign count := 1;
        perform action performIt : Notify;
        action ifIt if count > 0 { action body1; }
        action whileIt while count < 3 { action body2; }
        action forIt for i : Integer in (1, 2, 3) { action body3; }
    }
    part def Vehicle {
        part tank : Tank;
        part engine : Engine;
        flow f of Fuel from tank.fuelOut to engine.fuelIn;
        succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;
    }
}
