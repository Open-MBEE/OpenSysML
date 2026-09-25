// Two sibling parts of one type carry the same behavioral feature: `send
// alert()` performed by left is left's alert, so it must not satisfy an accept
// subsetting right.alert, though both name the one feature Unit::alert.
package test {
	private import ScalarValues::*;

	item def Go;

	part def Unit {
		action alert;
	}

	action controller {
		attribute rightTaken : Integer = 0;

		part left : Unit {
			perform action fire {
				first start;
				action reply send alert() to interrupt;
				done;
				succession first start then reply;
				succession first reply then done;
			}
		}
		part right : Unit;

		first start;
		// Addressing left materializes it, and its fire runs as it starts.
		action poke send new Go() to left;
		action interrupt accept :> right.alert;
		action record {
			assign rightTaken := 1;
		}
		done;

		succession first start then poke;
		succession first poke then interrupt;
		succession first interrupt then record;
		succession first record then done;
	}
}
