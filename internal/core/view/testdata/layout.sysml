package Plant {
	private import DiagramLayout::*;

	port def FluidPort;

	part def Pump {
		port outlet : FluidPort;
	}

	part def Tank {
		port inlet : FluidPort;
	}

	part def Loop {
		// The inline position, which a view positions over.
		part pump : Pump { @Layout { x = 10; y = 20; width = 100; height = 50; } }
		part tank : Tank;
		connection supply connect pump.outlet to tank.inlet {
			@Route { points = (60, 45, 200, 45); }
		}
	}

	state def Machine {
		entry; then off;
		state off;
		state on;
		transition off_on first off then on { @Route { points = (50, 10, 50, 90); } }
		transition first on then off;
	}

	action def Drive {
		first start;
		action provide { @Layout { x = 20; y = 30; } }
		action park;
		succession first start then provide;
		succession first provide then park { @Route { points = (20, 60, 20, 120); } }
	}
}

package PlantViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view placedView {
		expose Plant::Loop;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 1200; height = 800; }
		metadata Layout about Plant::Loop::pump { x = 300; y = 40; collapsed = true; }
		metadata Layout about Plant::Loop::tank { x = 500; y = 40; width = 120; height = 60; }
		metadata Route about Plant::Loop::supply { points = (400, 70, 450, 120, 500, 70); }
	}

	// Positions nothing of its own: pump keeps its inline position, tank has none.
	view plainView {
		expose Plant::Loop;
		render asInterconnectionDiagram;
	}

	view driveView : ActionFlowView {
		expose Plant::Drive;
		metadata Layout about Plant::Drive::park { x = 20; y = 150; }
	}

	view machineView : StateTransitionView {
		expose Plant::Machine;
		metadata Layout about Plant::Machine::off { x = 0; y = 0; }
		metadata Layout about Plant::Machine::on { x = 0; y = 100; width = 80; height = 40; }
	}
}
