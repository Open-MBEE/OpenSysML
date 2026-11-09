package Fuel {
	port def FuelPort { out item fuel; }
	interface def FuelLine { end supplier : FuelPort; end consumer : ~FuelPort; }
	part def Tank { port fuelOut : FuelPort; }
	part def Engine { port fuelIn : ~FuelPort; }
	part def Powertrain { part tank : Tank; part engine : Engine; }
	part def Wheel;
	part vehicle {
		part powertrain : Powertrain;
		part wheels : Wheel[2];
		ref part pump : Tank = powertrain.tank;
		interface fuelLine : FuelLine connect supplier ::> powertrain.tank.fuelOut to consumer ::> powertrain.engine.fuelIn;
	}
}
