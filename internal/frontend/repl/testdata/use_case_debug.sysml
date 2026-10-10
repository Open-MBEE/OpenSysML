// Use cases the action debugger runs: a usage, a definition, and one including another.
package Drones {
	part def Drone { attribute mass = 3.5; }
	part d : Drone;

	use case chargeDrone {
		subject drone : Drone = d;
		attribute charge = 0;

		first start;
		then action plugDrone { assign charge := charge + 1; }
		then action chargeDroneBatteries { assign charge := charge + 10; }
		then done;
	}

	use case def Charging {
		subject drone : Drone = d;
		attribute charge = 0;

		first start;
		then action plugDrone { assign charge := charge + 1; }
		then done;
	}

	use case def Weigh {
		subject drone : Drone = d;
		out m;

		first start;
		then action read { assign m := drone.mass; }
		then done;
	}

	use case mission {
		subject drone : Drone = d;
		out total;

		first start;
		then include use case charge : Charging;
		then include use case weigh : Weigh;
		then action sum { assign total := charge.charge + weigh.m; }
		then done;
	}
}
