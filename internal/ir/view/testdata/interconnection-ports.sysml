package ToasterDemo {
	private import ScalarValues::*;

	port def DurationPort {
		out duration : Real;
	}

	part def HeatingSystem {
		port durationIn : ~DurationPort;
	}

	part def ControlSystem {
		port durationOut : DurationPort;
	}

	// A definition with no port: a usage of it draws as a plain node.
	part def Chassis;

	part def Toaster {
		part heating : HeatingSystem;
		part control : ControlSystem;
		part chassis : Chassis;
		interface durationInterface connect control.durationOut to heating.durationIn;
		flow of Real from control.durationOut.duration to heating.durationIn.duration;
	}

	// Two usages of one definition each draw the definition's port as their own.
	part def Dual {
		part primary : ControlSystem;
		part backup : ControlSystem;
		part heating : HeatingSystem;
		connection standby connect backup.durationOut to heating.durationIn;
	}

	// A composite part — one nesting parts of its own — whose connector names a
	// port it has from its definition bare.
	part def Sensor {
		port reading : DurationPort;
	}

	part def Rig {
		part sensor : Sensor {
			part probe : HeatingSystem;
			connection feed connect reading to probe.durationIn;
		}
	}
}

package ToasterViews {
	private import Views::*;

	view toasterView {
		expose ToasterDemo::Toaster;
		render asInterconnectionDiagram;
	}

	view dualView {
		expose ToasterDemo::Dual;
		render asInterconnectionDiagram;
	}

	// The definition beside a usage of it: the usage's end is the usage's port.
	view besideView {
		expose ToasterDemo::HeatingSystem;
		expose ToasterDemo::Toaster;
		render asInterconnectionDiagram;
	}

	// The definition beside a composite usage of it: the bare end is the usage's pin.
	view rigView {
		expose ToasterDemo::Sensor;
		expose ToasterDemo::Rig;
		render asInterconnectionDiagram;
	}
}
