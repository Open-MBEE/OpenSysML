// A member of a multi-valued part that single-valued parts of the same owner also
// hold: a path reaches it through one of those, so it is not named by identity.
package Depot {
	part def Rover {
		attribute level = 0;

		exhibit state modes {
			entry; then waiting;
			state waiting;
		}
	}

	part def Garage {
		part bays : Rover[2];
		part front :> bays;
		part lead : Rover = bays#(1);
	}

	part garage : Garage;
}
