// A send addressed to a feature holding an object reaches the object it holds,
// not the feature's usage: `second` yields the Car the action wrote into it, so
// its machine hears the ping while the declared `car`'s stays waiting.
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
		part car : Car;
		part second : Car[0..1];

		perform action build {
			first start;
			then action make {
				assign second := new Car();
			}
			then action ping {
				send new Ping() to second;
			}
			then done;
		}
	}
}
