// An accept subsetting the event of an unselected variation names the
// declaration Unit::alert but evaluates to no occurrence: a sibling part's
// message from that same declaration must not satisfy it.
package test {
	private import ScalarValues::*;

	item def Ping;

	part def Unit {
		item alert : Ping;
	}

	action controller {
		attribute chosenTaken : Integer = 0;
		variation part chosen : Unit {
			variant part a : Unit;
			variant part b : Unit;
		}
		part right : Unit;

		first start;

		action finish {
			send right.alert to interrupt;
		}

		action interrupt accept :> chosen.alert;

		action record {
			assign chosenTaken := 1;
		}

		done;

		succession first start then finish;
		succession first finish then interrupt;
		succession first interrupt then record;
		succession first record then done;
	}
}
