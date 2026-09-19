// A later node reads what an earlier one wrote to the performance occurrence: the
// action's features are one space its whole flow shares.
package test {
	private import ScalarValues::*;

	action def Accumulate {
		attribute total : Integer = 0;
		out attribute doubled : Integer;

		action first_step {
			assign total := 5;
		}
		action second_step {
			assign doubled := total * 2;
		}
		first first_step;
		succession first first_step then second_step;
	}

	part def Adder {
		perform action sum : Accumulate;
	}
}
