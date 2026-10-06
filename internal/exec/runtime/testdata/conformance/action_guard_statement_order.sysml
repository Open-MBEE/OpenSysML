package test {
	private import ScalarValues::*;

	constraint def Ok {
		attribute y : Integer := 1;
		assign y := y * 10;
		assign y := y + 2;
		y == 12
	}

	action def DecisionGuard {
		out attribute r : Integer := 0;
		first start then choose;
		decide choose;
		if Ok() then yes;
		else no;
		action yes { assign r := 1; }
		then done;
		action no { assign r := 2; }
		then done;
	}
}
