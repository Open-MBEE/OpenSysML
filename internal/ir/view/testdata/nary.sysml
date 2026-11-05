package Nary {
	port def Plug;
	item def Fuel;
	item def Reading;

	part def Hub;
	part def Spoke;

	// A ternary connection definition: a dot with a segment to each end's type.
	connection def Triad {
		end hub : Hub [1];
		end left : Spoke [1];
		end right : Spoke [1];
	}

	part def Wheel {
		part hub : Hub;
		part leftSpoke : Spoke;
		part rightSpoke : Spoke;
		// A ternary connection usage: a dot named `spokes` with three segments.
		connection spokes : Triad connect (hub ::> hub, left ::> leftSpoke, right ::> rightSpoke);
		// An unnamed, untyped ternary connection: a dot with no label.
		connect (hub, leftSpoke, rightSpoke);
	}

	part def Tank {
		port fuelOut : Plug;
	}
	part def Engine {
		port fuelIn : Plug;
	}
	part def Car {
		part tank : Tank;
		part engine : Engine;
		// A connection whose line carries the flows between its two ports.
		connection fuelLine connect tank.fuelOut to engine.fuelIn;
		flow of Fuel from tank.fuelOut to engine.fuelIn;
		flow level of Reading from engine.fuelIn to tank.fuelOut;
		message of Reading from tank.fuelOut to engine.fuelIn;
		// A flow between two ports no connection joins stays an edge of its own.
		part gauge : Engine;
		flow of Reading from tank.fuelOut to gauge.fuelIn;
	}

}

package NaryRules {
	part def Chassis;
	part def Lights;
	part def Brakes;
	part def Steering;
	// An n-ary dependency: a dot named `rules`, a client link from each client and
	// a supplier link to each supplier.
	dependency rules from Brakes, Steering to Chassis, Lights;
}

package NaryViews {
	private import Views::*;
	private import DiagramLayout::*;
	private import StandardViewDefinitions::*;

	view wheelLinks {
		expose Nary::Wheel;
		render asInterconnectionDiagram;
		metadata Note about Nary::Wheel::spokes { text = "three spokes"; x = 10; y = 10; }
	}
	view carLinks {
		expose Nary::Car;
		render asInterconnectionDiagram;
		metadata Note about Nary::Car::fuelLine { text = "the line"; x = 10; y = 10; }
		metadata Note about Nary::Car::level { text = "the reading"; x = 10; y = 40; }
		metadata Note about Nary::Car::fuelLine, Nary::Car::level { text = "shared"; x = 10; y = 70; }
	}
	view definitionsRouted {
		expose Nary::Triad;
		expose Nary::Hub;
		expose Nary::Spoke;
		render asTreeDiagram;
		metadata Route about Nary::Triad { points = (10, 10, 20, 20); }
	}
	view dependenciesClients {
		expose NaryRules::Brakes;
		expose NaryRules::Steering;
		expose NaryRules::rules;
		render asTreeDiagram;
	}
	view definitions {
		expose Nary::Triad;
		expose Nary::Hub;
		expose Nary::Spoke;
		render asTreeDiagram;
	}
	view dependencies {
		expose NaryRules::Chassis;
		expose NaryRules::Lights;
		expose NaryRules::Brakes;
		expose NaryRules::Steering;
		expose NaryRules::rules;
		render asTreeDiagram;
	}
	view carLinksRouted {
		expose Nary::Car;
		render asInterconnectionDiagram;
		metadata Route about Nary::Car::fuelLine { points = (150, 80, 258, 247.5, 258, 121, 206, 121); }
	}
}
