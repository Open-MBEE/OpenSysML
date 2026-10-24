package TypedUsageViews {
	private import Views::*;

	port def Signal;

	abstract part def Component {
		attribute mass;
		part subcomponents : Component[*] default null;
	}

	part def Engine :> Component {
		attribute thrust;
	}

	part def Stage :> Component {
		attribute propellantMass;
		port status : Signal;
		part engines : Engine[*];
		part :>> subcomponents = engines;
	}

	part def Vehicle :> Component {
		attribute :>> mass = 0;
		port telemetry : Signal;
		part stage1 : Stage;
		part stage2 : Stage {
			attribute :>> propellantMass = 2;
		}
		part :>> subcomponents = (stage1, stage2);
		connect stage1.status to telemetry;
	}

	// A bodiless usage: everything it shows comes from its definition.
	part rocket : Vehicle;

	view rocketLinks {
		expose rocket;
		attribute depth = 3;
		render asInterconnectionDiagram;
	}

	view vehicleTree {
		expose Vehicle;
		render asTreeDiagram;
	}
}
