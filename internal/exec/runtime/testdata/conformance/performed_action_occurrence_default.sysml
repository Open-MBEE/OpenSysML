// A performed action starts from the occurrence's redefined slot value, rather
// than re-evaluating the definition's default.
package test {
	private import ScalarValues::*;

	action def Bump {
		attribute count : Integer = 1;

		action step {
			assign count := count + 1;
		}
		first step;
	}

	part def ConfiguredHost {
		perform action work : Bump {
			attribute redefines count = 5;
		}
	}
}
