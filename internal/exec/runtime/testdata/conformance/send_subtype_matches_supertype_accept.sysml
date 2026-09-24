// An accept typed by a supertype takes a message of a subtype: matching is by
// conformance, not by name equality.
package SendSubtypeAccept {
	private import ScalarValues::*;

	item def Command {
		attribute code : Integer;
	}

	item def Halt :> Command;

	port def CommandPort {
		out item issued : Command;
	}

	part def Console {
		port command : CommandPort;
		attribute sent : Integer = 0;

		exhibit state dispatch {
			entry; then idle;

			state idle;

			state sending {
				entry action issue {
					send new Halt(9) via command;
					assign sent := sent + 1;
				}
			}

			succession first idle then sending;
		}
	}

	part def Unit {
		port command : ~CommandPort;
		attribute accepted : Integer = 0;
		attribute lastCode : Integer = 0;

		exhibit state duty {
			entry; then standingBy;

			state standingBy;

			state working {
				entry action count {
					assign accepted := accepted + 1;
				}
			}

			transition first standingBy
				accept order : Command via command
				do assign lastCode := order.code
				then working;
		}
	}

	part def Site {
		part console : Console;
		part unit : Unit;

		connect console.command to unit.command;
	}

	part site : Site;
}
