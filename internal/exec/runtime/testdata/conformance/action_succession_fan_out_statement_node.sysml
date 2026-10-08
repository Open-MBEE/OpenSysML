package test {
	private import ScalarValues::*;

	// A statement node is a step like any other: two successions out of an `if`
	// node are two HappensBefore links, so both b and c follow it.
	action branches {
		attribute x : Integer = 0;
		attribute bRan : Boolean = false;
		attribute cRan : Boolean = false;

		first start;
		then if x == 0 { assign x := 1; }
		then b;
		then c;
		action b { assign bRan := true; }
		action c { assign cRan := true; }
	}
}
