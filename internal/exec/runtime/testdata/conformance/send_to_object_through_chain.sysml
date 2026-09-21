// A send addressed through a chain reaches the object the chain ends at:
// `garage.cars#(2)` walks the held garage to the second object its cars hold,
// so that car's machine hears the ping while the first's stays waiting.
package test {
	item def Ping;

	part def Car {
		exhibit state listening {
			entry; then waiting;

			state waiting;
			accept Ping then heard;
			state heard;
		}
	}

	part def Garage {
		part cars : Car[0..*];
	}

	part def Fleet {
		part garage : Garage;

		perform action build {
			first start;
			then action make {
				assign garage.cars := (garage.cars, new Car());
				assign garage.cars := (garage.cars, new Car());
			}
			then action ping {
				send new Ping() to garage.cars#(2);
			}
			then done;
		}
	}
}
