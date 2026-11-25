package AnonymousConnectors {
	private import Views::*;

	port def Fuel;

	part def Vehicle {
		part engine {
			port fuelIn : Fuel;
			port shaftOut : Fuel;
		}
		part tank {
			port fuelOut : Fuel;
		}
		part wheel {
			port hub : Fuel;
		}
		connect tank.fuelOut to engine.fuelIn;
		connect engine.shaftOut to wheel.hub;
		connect engine.fuelIn to wheel.hub;
	}

	view wiring {
		expose Vehicle;
		render asInterconnectionDiagram;
	}
}
