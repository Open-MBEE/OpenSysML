// An entry accept's payload is what the body's entry transitions read: `sizing`
// parks at its accept of Count, and its guards choose `big` from the `n` the
// entry assigned once Count has arrived, not from the 0 it held when entered.
package StateEntryAcceptGuardReadsEntry {
	private import ScalarValues::*;

	item def Go;
	item def Count {
		attribute value : Integer;
	}
	item def Start;
	item def Ack;

	port def SenderPort {
		out item go : Go;
		out item count : Count;
		in item ack : Ack;
	}

	port def ReceiverPort {
		in item go : Go;
		in item count : Count;
		out item ack : Ack;
	}

	part def Receiver {
		port p : ReceiverPort;
		attribute n : Integer = 0;
		item ack : Ack;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go via p then sizing;
			state sizing {
				entry action {
					send ack via p;
					then accept c : Count via p;
					then assign n := c.value;
				}
				if n > 5 then big;
				if n <= 5 then small;
				state big;
				state small;
			}
		}
	}

	part def Sender {
		port out : SenderPort;
		item go : Go;
		item seven : Count {
			attribute :>> value = 7;
		}

		exhibit state life {
			entry; then idle;
			state idle;
			state begin {
				entry send go via out;
			}
			state follow {
				entry send seven via out;
			}
			state sent;
			transition first idle accept Start then begin;
			transition first begin accept Ack via out then follow;
			transition first follow then sent;
		}
	}

	part def Rig {
		part sender : Sender;
		part target : Receiver;
		connect sender.out to target.p;
	}
}
