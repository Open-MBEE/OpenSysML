package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	action def Nap {
		first start;
		then action rest accept after 10 [s];
		then done;
	}

	// As action_terminate_names_node_of_paused_bodies, but slow naps by performing an
	// action of its own rather than running a flow: no token runs inside slow, so a
	// paused performance of it is held only where its token's body paused. Both still
	// end at `terminate slow;`, their naps abandoned, and both bodies go on past slow.
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
					perform action nap : Nap;
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
