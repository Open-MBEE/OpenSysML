package test {
	private import ScalarValues::*;

	action def TargetOnly {
		attribute x : Integer = 0;
		attribute sum : Integer = 0;

		first start then p;
		action p { assign x := 10; }
		then [3] a;
		action a[3] { assign sum := sum + x; }
		then done;
	}
}
