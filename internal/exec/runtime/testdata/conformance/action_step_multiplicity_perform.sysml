package test {
	private import ScalarValues::*;

	action def PerformTwice {
		attribute c : Integer = 0;

		action def Increment {
			attribute c : Integer = 0;
			first start then increment;
			action increment { assign c := c + 1; }
			then done;
		}

		first start then p;
		perform action p[2] : Increment;
		then done;
	}
}
