package Plant {
	port def FluidPort;

	part def Pump {
		port outlet : FluidPort;
		out item waterOut : Water;
	}

	part def Tank {
		port inlet : FluidPort;
		in item waterIn : Water;
	}

	item def Water;

	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply : Connection connect pump.outlet to tank.inlet;
		flow of Water from pump.waterOut to tank.waterIn;
		// An end outside the exposed set, reported rather than drawn.
		part sensor : Pump;
	}

	connection def Connection;
}

package PlantViews {
	private import Views::*;

	view loopView {
		expose Plant::Loop;
		render asInterconnectionDiagram;
	}
}
