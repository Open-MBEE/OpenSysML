// An object a feature classifies takes on the relationships its classifier declares,
// not only its features: the binding, subsetting and connector `Site` writes hold for
// the untyped `raw` once `site : Site = raw` makes it a Site — so its console sends
// through Site's connection and reaches its unit.
package test {
	private import ScalarValues::*;

	item def Command {
		attribute code : Integer;
	}

	port def CommandPort {
		out item issued : Command;
	}

	part def Console {
		port command : CommandPort;

		exhibit state dispatch {
			entry; then idle;
			state idle;
			state sending {
				entry action issue { send new Command(7) via command; }
			}
			succession first idle then sending;
		}
	}

	part def Unit {
		port command : ~CommandPort;
		attribute lastCode : Integer = 0;

		exhibit state duty {
			entry; then standingBy;
			state standingBy;
			state working;
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

		attribute label : String;
		attribute name : String = "north";
		bind label = name;

		attribute codes : Integer [*];
		attribute primary : Integer [*] :> codes = (7, 8);
	}

	part def Campus {
		part raw [1];
		part site : Site = raw;
	}

	part campus : Campus;
}
