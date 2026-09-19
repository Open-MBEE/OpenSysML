package test {
	private import ScalarValues::*;

	// Two fork branches both lead to the terminate action usage `stop`, which
	// synchronizes their arrivals; the token that then performs it ends the action.
	action joined {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		first start;
		then fork split;
			then a1;
			then a2;
		action a1 { assign x := 1; }
		then stop;
		action a2 { assign y := 2; }
		then stop;
		action stop terminate;
		then action after { assign z := 3; }
		then done;
	}
}
