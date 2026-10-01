// A step of a case body may wait on the clock (`accept after`): the case's own
// flow advances the clock to the instant, as a performed action's does, so the
// steps after the wait run at that instant instead of the body deadlocking.
package test {
	private import ScalarValues::*;
	private import SI::*;

	part def Ship {
		attribute log : Real default = 0.0;
		attribute arrived : Real default = -1.0;
	}
	part ship : Ship;

	analysis def Voyage {
		subject boat : Ship;
		in leg : Real;
		action depart { assign boat.log := boat.log + 1.0; }
		then action sail accept after leg [s];
		then action arrive {
			assign boat.log := boat.log + 1.0;
			assign boat.arrived := localClock.currentTime;
		}
		then action rest accept after 30.0 [s];
		return total : Real = boat.log;
		out arrival : Real = boat.arrived;
	}
	analysis voyage : Voyage {
		subject boat = ship;
		in leg = 60.0;
	}
}
