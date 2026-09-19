// Two parts joined by a connector their owner declares. The console sends
// through its own port, and the message arrives at the unit's port, where that
// object's machine accepts it: the connection crosses object identities.
package SendOwnerConnection {
	private import ScalarValues::*;

	item def Command {
		attribute code : Integer;
	}

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
					send new Command(7) via command;
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
