package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	action def Nap {
		first start;
		then action rest accept after 10 [s];
		then done;
	}

	// As action_terminate_names_node_of_paused_bodies, but slow naps by performing Nap:
	// each `terminate slow;` ends its own loop body's slow, abandoning that Nap.
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
				action inner {
					first start;
					then fork f;
					succession first f then nap;
					succession first f then watch;
					action nap : Nap;
					action watch accept after 5 [s];
					succession first watch then stop;
					action stop { terminate slow; }
				}
				assign napped := napped + 1;
			}
			assign passed := passed + 1;
			assign later := later + 1;
		} until true;
		then done;
	}
}
