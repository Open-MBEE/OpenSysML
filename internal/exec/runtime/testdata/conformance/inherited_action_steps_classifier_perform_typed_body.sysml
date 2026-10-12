package test {
	private import ScalarValues::*;

	part def Machine {
		attribute hits : Integer = 0;
		action def BumpBase {
			inout n : Integer;
			first start then a;
			action a { assign n := n + 1; }
			then done;
		}
		action def Bump :> BumpBase;
		perform action p : Bump {
			inout n = hits;
			action b { assign n := n + 100; }
		}
	}
}
