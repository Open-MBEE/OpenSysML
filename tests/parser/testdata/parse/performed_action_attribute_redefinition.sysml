package test {
	action def Bump {
		attribute count : Integer = 1;
		out attribute n : Integer;
		action step {
			assign count := count + 1;
			assign n := count * 2;
		}
		first step;
	}

	part def ConfiguredHost {
		perform action work : Bump {
			attribute redefines count = 5;
		}
	}
}
