// An accept subsetting a behavioral feature by its bare name takes the accepting
// object's own performance of it: left's listener hears left's `send alert()`,
// while right's listener, sent the same alert by left, keeps waiting.
package test {
	private import ScalarValues::*;

	part def Unit {
		attribute heard : Integer = 0;
		action alert;

		perform action listen {
			first start;
			action wait accept :> alert;
			action record {
				assign heard := 1;
			}
			done;
			succession first start then wait;
			succession first wait then record;
			succession first record then done;
		}
	}

	part def Group {
		part left : Unit {
			perform action fire {
				first start;
				action toSelf send alert();
				action toRight send alert() to right;
				done;
				succession first start then toSelf;
				succession first toSelf then toRight;
				succession first toRight then done;
			}
		}
		part right : Unit;
	}
}
