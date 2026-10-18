package test {
	private import ScalarValues::*;

	part def Node {
		attribute id : Integer = 7;
	}
	item def Payload {
		attribute mass : Real = 2.5;
	}
	action def Step;

	part def Rig {
		part own : Node;
		part spare : Node {
			:>> id = 9;
		}
		item cargo : Payload;
		action go : Step;

		ref part viaOwn ::> own;
		ref part viaSpare ::> spare;
		ref item viaCargo ::> cargo;
		ref action viaGo ::> go;
		ref part unbound : Node;
	}

	part rig : Rig;
}
