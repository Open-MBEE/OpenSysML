package test {
	private import ScalarValues::*;

	part def Machine {
		attribute hits : Integer = 0;
		action def BumpBase {
			first start then a;
			action a { assign hits := hits + 1; }
			then done;
		}
		action def Bump :> BumpBase;
		perform action p : Bump {
			action b { assign hits := hits + 100; }
		}
	}
}
