// A send addressed by an expression reaches the object the expression yields:
// `cars#(2)` is the second object `cars` holds, so its machine hears the ping
// while the first's, accepting the same signal, stays waiting.
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

	part def Fleet {
		part cars : Car[0..*];

		perform action build {
			first start;
			then action make {
				assign cars := (cars, new Car());
				assign cars := (cars, new Car());
			}
			then action ping {
				send new Ping() to cars#(2);
			}
			then done;
		}
	}
}
