package Test {
	private import ScalarValues::*;

	attribute def Stop;

	// The do behavior is a typed action whose flow is two steps. With `Stop` in the
	// pool as `top` is entered, the dispatch may cut the performance at any of its
	// moves, before its `inout` writes back, or take it after it ended.
	state Machine {
		attribute count : Integer = 0;

		action def Work {
			inout n : Integer;
			first start;
			then action a assign n := n + 1;
			then action b assign n := n + 10;
			then done;
		}

		entry; then top;
		state top {
			do action work : Work { inout n = count; }
		}
		transition first top accept Stop do assign count := count + 100 then idle;
		state idle;
	}
}
