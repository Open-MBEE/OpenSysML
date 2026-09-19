// Two sibling parts of one type expose the same event declaration: a message
// sent from right.alert must not satisfy an accept subsetting left.alert, though
// both name the one feature Unit::alert.
package test {
	private import ScalarValues::*;

	item def Ping;

	part def Unit {
		item alert : Ping;
	}

	action controller {
		attribute leftTaken : Integer = 0;
		part left : Unit;
		part right : Unit;

		first start;

		action finish {
			send right.alert to interrupt;
		}

		action interrupt accept :> left.alert;

		action record {
			assign leftTaken := 1;
		}

		done;

		succession first start then finish;
		succession first finish then interrupt;
		succession first interrupt then record;
		succession first record then done;
	}
}
