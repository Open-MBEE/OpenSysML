package test {
	private import ScalarValues::*;

	action def Ordered {
		attribute x : Integer = 0;
		attribute sum : Integer = 0;
		attribute observed : Integer = 0;
		action p { assign x := 10; }
		action a[3] { assign sum := sum + x; }
		action q { assign observed := sum; }
		succession first start then p;
		succession first [1] p then [*] a;
		succession first [*] a then [1] q;
		succession first q then done;
	}
}
