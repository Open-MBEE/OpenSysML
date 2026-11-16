// A loop makes a definition run by no object an occurrence as `this` does: node performances
// begun after it are all kept (`one`, three), of those ended before the latest per node (`early`).
package test {
	private import ScalarValues::*; private import SequenceFunctions::*;

	action def Pipeline {
		out attribute before : Integer[1] = 0;
		out attribute count : Integer[1] = 0;
		attribute n : Integer[1] = 0;
		action early { out v : Integer[1]; assign v := 5; }
		action warm {
			first start;
			then while n < 1 { assign n := n + 1; }
			then done;
		}
		action one { out v : Integer[1]; assign v := n; }
		action bump { assign n := n + 1; }
		action fin { assign before := size(this.early); assign count := size(this.one); }
		first start then early;
		first early then warm;
		first warm then m;
		merge m;
		first m then one;
		first one then bump;
		first bump then d;
		decide d;
		if n < 4 then m;
		else fin;
		first fin then done;
	}
}
