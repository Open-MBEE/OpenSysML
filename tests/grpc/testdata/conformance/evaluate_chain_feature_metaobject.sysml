// The anonymous Feature a chained connector end references (KerML 8.3.3.3.9)
// crosses as a metaobject naming its chaining features and the end owning it,
// and the connector's derived sourceFeature is that same Feature (8.3.4.5.3).
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
