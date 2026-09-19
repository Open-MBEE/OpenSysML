// A send addressed to one sibling reaches that sibling's exhibited machine and
// no other: `b` hears the ping its sibling `a` addressed to it while `c`, whose
// machine accepts the same signal, stays waiting.
package test {
	item def Ping;

	part def Listener;

	part def Group {
		part a : Listener {
			exhibit state sending {
				entry; then sent;

				state sent {
					entry send new Ping() to b;
				}
			}
		}

		part b : Listener {
			exhibit state listening {
				entry; then waiting;

				state waiting;
				accept Ping then heard;
				state heard;
			}
		}

		part c : Listener {
			exhibit state listening {
				entry; then waiting;

				state waiting;
				accept Ping then heard;
				state heard;
			}
		}
	}
}
