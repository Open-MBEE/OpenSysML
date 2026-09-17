// `terminate A` names the whole action: the branch waiting on the clock is
// cancelled with it, its assignment never runs, and the outputs already
// written stand.
package test {
	private import ScalarValues::*;

	action def A {
		out x : Integer := 0;
		out y : Integer := 0;

		first start;
		fork f;
		action a { assign x := 1; terminate A; assign x := 8; }
		action bw accept after 5 [SI::s];
		action b { assign y := 2; }
		join j;
		done;

		succession first start then f;
		succession first f then a;
		succession first f then bw;
		succession first bw then b;
		succession first a then j;
		succession first b then j;
		succession first j then done;
	}
}
