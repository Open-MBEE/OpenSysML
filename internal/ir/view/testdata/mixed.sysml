package MixedExamples {
	private import OpenSysMLRenderings::*;
	private import DiagramLayout::*;

	port def FluidPort;
	part def BaseUnit;

	part def Pump :> BaseUnit {
		port outlet : FluidPort;
	}

	part def Tank :> BaseUnit {
		port inlet : FluidPort;
	}

	part def Plant :> BaseUnit {
		part pump : Pump;
		part tank : Tank;
		connection supply : Connection connect pump.outlet to tank.inlet;
	}

	connection def Connection;

	state def Ready;

	state def Modes {
		entry; then ready;
		state ready : Ready;
	}

	action def Operate {
		first start;
		action step;
		done;
		succession first start then step;
		succession first step then done;
	}

	use case def Inspection {
		actor operator : Person;
		subject plant : Plant;
		objective {
			doc /* Inspect the plant and report its operating state. */
		}
		perform action run : Operate;
		exhibit state mode : Modes;
	}

	part def Person;

	view mixedDiagram {
		expose MixedExamples;
		render asMixedDiagram;
	}

	view mixedLayout {
		expose MixedExamples;
		render asMixedDiagram;
		metadata Layout about MixedExamples::Inspection { x = 50; y = 30; }
		metadata Layout about MixedExamples::Inspection::operator { x = 20; y = 80; }
		metadata Route about MixedExamples::Plant::pump { points = (100, 40, 140, 80); }
	}
}
