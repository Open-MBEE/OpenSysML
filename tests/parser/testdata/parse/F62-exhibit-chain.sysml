package F62Exhibit {
	state def VehicleStates {
		state on;
		state off;
	}
	part def Vehicle {
		exhibit state vehicleStates : VehicleStates;
	}
	part vehicle : Vehicle {
		exhibit vehicleStates.on {
			doc /* body */
		}
	}
}
