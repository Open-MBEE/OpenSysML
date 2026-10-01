package Shared {
	part def Sensor;
	part def Rig {
		part left : Sensor;
		part right : Sensor;
		part spare : Sensor;
		part backup : Sensor;
	}
}

package SharedViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	// One Note about two parts is one comment anchored to both; the two Notes
	// stated apart on spare and backup are two comments, alike as they are.
	view rigView {
		expose Shared::Rig;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 800; height = 400; }
		metadata Layout about Shared::Rig::left { x = 20; y = 20; width = 100; height = 50; }
		metadata Layout about Shared::Rig::right { x = 200; y = 20; width = 100; height = 50; }
		metadata Layout about Shared::Rig::spare { x = 20; y = 200; width = 100; height = 50; }
		metadata Layout about Shared::Rig::backup { x = 200; y = 200; width = 100; height = 50; }
		metadata Note about Shared::Rig::left, Shared::Rig::right { text = "calibrated"; x = 400; y = 20; width = 120; height = 40; }
		metadata Note about Shared::Rig::spare { text = "on loan"; x = 400; y = 200; width = 120; height = 40; }
		metadata Note about Shared::Rig::backup { text = "on loan"; x = 400; y = 200; width = 120; height = 40; }
	}
}
