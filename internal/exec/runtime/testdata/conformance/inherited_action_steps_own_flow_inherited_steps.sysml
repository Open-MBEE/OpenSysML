package test {
	private import ScalarValues::*;
	action def G {
		action a { out y : Integer; assign y := 7; }
		action b { in v : Integer; }
		first start then a;
		then b;
		then done;
	}
	action def S :> G {
		flow f2 from a.y to b.v;
	}
}
