package Test {
	private import ScalarValues::*;

	attribute def Stop;

	// A do behavior's flow performs a typed action, `w : Work`, whose own flow is
	// two steps. With `Stop` in the pool as `top` is entered, the dispatch may cut
	// the do behavior before `mark`, between `mark` and the performance, at any
	// move inside `Work` (whose `inout` writes back only when it ends, so `count`
	// still reads 1), after it, or after `tail`.
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
			do action run {
				first start;
				then action mark assign count := 1;
				then perform action w : Work { inout n = count; }
				then action tail assign count := count + 100;
				then done;
			}
		}
		transition first top accept Stop do assign count := count + 1000 then idle;
		state idle;
	}
}
