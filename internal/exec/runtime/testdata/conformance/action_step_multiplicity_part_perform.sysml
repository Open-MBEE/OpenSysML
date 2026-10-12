// `perform action run[2]` on a part names two performances the part enacts in its
// lifetime (Parts::Part::performedActions, Occurrences::enactedPerformances),
// unordered with each other; each adds one to the part's `count`.
package test {
	private import ScalarValues::*;

	part def Host {
		attribute count : Integer = 0;
		exhibit state life {
			entry; then idle;
			state idle;
		}
		perform action run[2] {
			action step { assign count := count + 1; }
			first step;
		}
	}
}
