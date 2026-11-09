// A `succession flow` is a successionFlowUsage; the plain `flow` beside it
// stays a flowUsage.
package P {
	item def Fuel;
	part def Tank { out item fuelOut : Fuel; }
	part def Engine { in item fuelIn : Fuel; }
	part def Vehicle {
		part tank : Tank;
		part engine : Engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;
	}
}
