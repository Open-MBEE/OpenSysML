// A connector end reached through a multi-valued feature denotes every element
// it holds, so one send through the console's port delivers one message per
// unit, each on that element's own identity.
package SendFanOut {
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
		part units : Unit[2];

		connect console.command to units.command;
	}

	part site : Site;
}
