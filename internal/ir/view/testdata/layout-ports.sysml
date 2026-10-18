// Ports a Layout places as the source tool drew them: a rectangle sized to carry the port's
// own name, over the border of the part. The rendering keeps the rectangle's position alone and
// draws the notation's port square there.
package LayoutPorts {
	private import Views::*;
	private import DiagramLayout::*;

	port def Signal;
	part def Detector { port tx : Signal { attribute level; } }
	part def Amplifier { port rx : ~Signal; }
	part def Chain {
		part detector : Detector;
		part amplifier : Amplifier;
		connection link connect detector.tx to amplifier.rx;
	}

	// The ports the parts have from their definitions, exposed beside the parts, each drawn
	// over the border between the two parts.
	view typed {
		expose Detector::tx;
		expose Amplifier::rx;
		expose Chain::detector;
		expose Chain::amplifier;
		expose Chain::link;
		render asInterconnectionDiagram;
		metadata Layout about Chain::detector { x = 40; y = 20; width = 400; height = 160; }
		metadata Layout about Chain::amplifier { x = 40; y = 240; width = 400; height = 160; }
		metadata Layout about Detector::tx { x = 200; y = 165; width = 130; height = 36; }
		metadata Layout about Amplifier::rx { x = 200; y = 219; width = 130; height = 36; }
		metadata Route about Chain::link { points = (265, 201, 265, 219); }
	}

	part def Bench {
		part supply { port outlet; }
		part instrument {
			port inlet;
			part sensor { port probe; }
		}
		connection feed connect supply.outlet to instrument.inlet;
	}

	// Declared ports, drawn as members of their parts: the supply's over its right border, the
	// instrument's over its left border, and the sensor's inside the sensor near its bottom.
	view declared {
		expose Bench::supply;
		expose Bench::instrument;
		expose Bench::feed;
		render asInterconnectionDiagram;
		metadata Layout about Bench::supply { x = 40; y = 20; width = 200; height = 100; }
		metadata Layout about Bench::supply::outlet { x = 175; y = 60; width = 130; height = 36; }
		metadata Layout about Bench::instrument { x = 400; y = 20; width = 300; height = 200; }
		metadata Layout about Bench::instrument::inlet { x = 335; y = 42; width = 130; height = 36; }
		metadata Layout about Bench::instrument::sensor { x = 450; y = 80; width = 200; height = 100; }
		metadata Layout about Bench::instrument::sensor::probe { x = 500; y = 150; width = 130; height = 36; }
		metadata Route about Bench::feed { points = (305, 78, 335, 60); }
	}

	// A definition's port in a tree rendering, drawn over the definition's right border; the
	// attribute nested in the port has no place and is left undrawn.
	view tree {
		expose Detector;
		render asTreeDiagram;
		metadata Layout about Detector { x = 40; y = 20; width = 200; height = 100; }
		metadata Layout about Detector::tx { x = 175; y = 100; width = 130; height = 36; }
	}

	part def Probe {
		port tap : Signal;
		part sensor;
		connection wire connect tap to sensor;
	}
	part def Solo { part probe : Probe; }
	part def Rig {
		part primary : Probe;
		part backup : Probe;
	}

	// One usage of the definition beside the definition's port, nested part and the connection
	// naming the port bare: the port is the usage's pin, where the connection ends.
	view soloWire {
		expose Solo::probe;
		expose Probe::tap;
		expose Probe::sensor;
		expose Probe::wire;
		render asInterconnectionDiagram;
		metadata Layout about Solo::probe { x = 40; y = 20; width = 300; height = 200; }
		metadata Layout about Probe::tap { x = 275; y = 82; width = 130; height = 36; }
		metadata Layout about Probe::sensor { x = 100; y = 80; width = 120; height = 60; }
		metadata Route about Probe::wire { points = (220, 110, 275, 100); }
	}

	// Two usages of the definition beside its port: the bare name is neither usage's pin, so the
	// port keeps a square of its own, which the connection reaches.
	view pairWire {
		expose Rig::primary;
		expose Rig::backup;
		expose Probe::tap;
		expose Probe::sensor;
		expose Probe::wire;
		render asInterconnectionDiagram;
		metadata Layout about Rig::primary { x = 40; y = 20; width = 300; height = 200; }
		metadata Layout about Rig::backup { x = 400; y = 20; width = 300; height = 200; }
		metadata Layout about Probe::tap { x = 275; y = 82; width = 130; height = 36; }
		metadata Layout about Probe::sensor { x = 100; y = 80; width = 120; height = 60; }
		metadata Route about Probe::wire { points = (220, 110, 275, 100); }
	}

	// The definition's port exposed beside the usage with nothing connected to it.
	view bare {
		expose Solo::probe;
		expose Probe::tap;
		render asInterconnectionDiagram;
		metadata Layout about Solo::probe { x = 40; y = 20; width = 300; height = 200; }
		metadata Layout about Probe::tap { x = 275; y = 82; width = 130; height = 36; }
	}

	part def Hub {
		port bus : Signal {
			port lane : Signal;
		}
	}

	// A port nested in a port, both sized, in a tree: the inner square touches the outer square.
	view nested {
		expose Hub;
		render asTreeDiagram;
		metadata Layout about Hub { x = 40; y = 20; width = 200; height = 100; }
		metadata Layout about Hub::bus { x = 175; y = 100; width = 130; height = 36; }
		metadata Layout about Hub::bus::lane { x = 200; y = 118; width = 60; height = 20; }
	}
}
