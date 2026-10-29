// Two parts whose ports sit on their top borders, under the header of the
// part that holds them: the connection between the ports has to leave each
// port upward, into the strip between the header and the parts.
package PumpStation {
	port def FluidPort;

	part def Pump {
		port outlet : FluidPort;
	}

	part def Tank {
		port inlet : ~FluidPort;
	}

	part def Station {
		part supply : Pump;
		part reservoir : Tank;
		connection fluid connect supply.outlet to reservoir.inlet;
	}

	part station : Station;
}

package PumpStationViews {
	private import Views::*;
	private import DiagramLayout::*;

	view stationView {
		expose PumpStation::station;
		render asInterconnectionDiagram;
		metadata Layout about PumpStation::station { x = 40; y = 40; }
		metadata Layout about PumpStation::station::supply { x = 80; y = 220; }
		metadata Layout about PumpStation::station::reservoir { x = 300; y = 220; }
	}
}
