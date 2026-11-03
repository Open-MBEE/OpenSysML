package FoldedPorts {
	private import Views::*;
	private import DiagramLayout::*;

	port def Fuel;

	part def Vehicle {
		part engine {
			port fuelIn : Fuel;
			port shaftOut : Fuel;
			part injector {
				port feed : Fuel;
				attribute rate : ScalarValues::Real;
			}
			attribute mass : ScalarValues::Real;
		}
		part tank {
			port fuelOut : Fuel;
		}
		part wheel {
			port hub : Fuel;
		}
		connect tank.fuelOut to engine.fuelIn;
		connect engine.shaftOut to wheel.hub;
		connect engine.injector.feed to tank.fuelOut;
	}

	// A collapsed Layout folds engine: its ports stay on its boundary, and the
	// connectors still end at them; injector and what injector nests are not drawn.
	metadata Layout about Vehicle::engine { collapsed = true; }

	view tree {
		expose Vehicle;
		render asTreeDiagram;
	}

	view wiring {
		expose Vehicle;
		render asInterconnectionDiagram;
	}
}
