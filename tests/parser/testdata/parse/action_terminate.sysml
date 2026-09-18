package test {
	private import ScalarValues::*;

	action def Guarded {
		attribute x : Integer = 0;

		first start;
		then action c1 {
			assign x := 1;
			terminate c1;
		}
		then action c2 {
			assign x := 2;
			terminate;
		}
		then terminate;
		then action late { assign x := 3; }
		then done;
	}

	action def Stopped {
		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate;
		then action a2 terminate;
	}
}
