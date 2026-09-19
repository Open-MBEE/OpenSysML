package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	// Three tokens reach the loop node through the merge a step apart, each running its
	// body. The first two nap in a flow inside slow, so two performances of slow are
	// paused at once, each held where its token's body paused, while the third skips
	// the nap and its `terminate slow;` names the node: both paused performances end,
	// their naps dropped and neither later assignment run, and both bodies go on past slow.
	action host {
		out attribute entered : Integer = 0;
		out attribute napped : Integer = 0;
		out attribute passed : Integer = 0;
		out attribute later : Integer = 0;

		fork split;
		action pre;
		action pre1;
		action pre2;
		succession first start then split;
		succession first split then gate;
		succession first split then pre;
		succession first pre then gate;
		succession first split then pre1;
		succession first pre1 then pre2;
		succession first pre2 then gate;
		merge gate;
		then loop {
			action slow {
				assign entered := entered + 1;
				if entered <= 2 {
					action inner {
						first start;
						then action nap accept after 10 [s];
						then done;
					}
					assign napped := napped + 1;
				}
			}
			assign passed := passed + 1;
			if passed == 1 {
				terminate slow;
			}
			assign later := later + 1;
		} until true;
		then done;
	}
}
