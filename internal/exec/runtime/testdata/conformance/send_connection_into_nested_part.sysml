// A part joining its own port to the port of a part it holds delegates inward,
// so the send arrives at the nested part on that part's own identity.
package SendDelegation {
	private import ScalarValues::*;

	item def Command {
		attribute code : Integer;
	}

	port def CommandPort {
		out item issued : Command;
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

	part def Bay {
		port command : CommandPort;
		attribute sent : Integer = 0;

		part unit : Unit;

		connect command to unit.command;

		exhibit state dispatch {
			entry; then idle;

			state idle;

			state sending {
				entry action issue {
					send new Command(4) via command;
					assign sent := sent + 1;
				}
			}

			succession first idle then sending;
		}
	}

	part bay : Bay;
}
