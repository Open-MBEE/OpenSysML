// A connector's ends and the features they connect, as a client reads them off
// the connector's symbol: an interface whose ends are feature chains.
package InterfaceEnds {
	port def FuelPort;
	part def Tank { port fuelOut : FuelPort; }
	part def Engine { port fuelIn : FuelPort; }
	part def Powertrain { part tank : Tank; part engine : Engine; }
	part vehicle {
		part powertrain : Powertrain;
		interface fuelLine connect
			supplier ::> powertrain.tank.fuelOut to
			consumer ::> powertrain.engine.fuelIn;
	}
}
