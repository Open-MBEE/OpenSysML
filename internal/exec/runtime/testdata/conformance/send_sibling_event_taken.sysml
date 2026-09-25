// Two sibling parts of one type expose the same event declaration: a message
// sent from right.alert satisfies an accept subsetting right.alert, the very
// occurrence the accept evaluates its path to.
package test {
	private import ScalarValues::*;

	item def Ping;

	part def Unit {
		item alert : Ping;
	}

	action controller {
		attribute rightTaken : Integer = 0;
		part left : Unit;
		part right : Unit;

		first start;

		action finish {
			send right.alert to interrupt;
		}

		action interrupt accept :> right.alert;

		action record {
			assign rightTaken := 1;
		}

		done;

		succession first start then finish;
		succession first finish then interrupt;
		succession first interrupt then record;
		succession first record then done;
	}
}
