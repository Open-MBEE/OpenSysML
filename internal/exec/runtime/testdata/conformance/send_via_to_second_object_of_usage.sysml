// A routed send whose `to` clause is an expression reaches the object it
// yields: the port `out` joins every car's `p`, and `cars#(2)` confines the
// copies to the second, so its machine hears the ping while the first's stays
// waiting.
package test {
	item def Ping;

	port def PingPort {
		in item ping : Ping;
	}

	part def Car {
		port p : PingPort;

		exhibit state listening {
			entry; then waiting;

			state waiting;
			accept Ping via p then heard;
			state heard;
		}
	}

	part def Fleet {
		part cars : Car[0..*];
		port out : ~PingPort;

		connect out to cars.p;

		perform action build {
			first start;
			then action make {
				assign cars := (cars, new Car());
				assign cars := (cars, new Car());
			}
			then action ping {
				send new Ping() via out to cars#(2);
			}
			then done;
		}
	}
}
