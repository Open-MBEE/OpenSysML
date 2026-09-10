// Two state machines materialized in one context read one clock. The receiver
// arms itself after 3 s; the sender pings after 5 s. On a shared clock the ping
// finds the receiver armed; on a clock of its own the receiver would still be
// waiting at its own t=0 and take the ping early.
package test {
	private import SI::*;

	attribute def Ping;

	part def Sender {
		part receiver : Receiver;

		exhibit state pinging {
			entry; then holding;

			state holding;
			accept after 5 [s] then pinged;
			state pinged {
				entry send new Ping() to receiver;
			}
		}
	}

	part def Receiver {
		exhibit state listening {
			entry; then waiting;

			state waiting;
			accept after 3 [s] then armed;
			accept Ping then early;
			state armed;
			accept Ping then gotPing;
			state early;
			state gotPing;
		}
	}

	part sender : Sender;
}
