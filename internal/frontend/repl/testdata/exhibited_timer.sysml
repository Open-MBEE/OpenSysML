// A part whose exhibited machine re-enters one state on a timer, its do action
// writing the part's own attributes each time.
package TA {
	private import ScalarValues::*;
	private import SI::*;

	part def Sys {
		attribute v : Real = 2.0;
		attribute x : Real = 0.0;
		attribute n : Integer = 0;

		exhibit state lp {
			entry; then run;
			state run {
				do action step {
					assign x := x + v * 0.1;
					assign n := n + 1;
				}
			}
			transition run_tick first run accept after 1 [s] then run;
		}
	}
}
