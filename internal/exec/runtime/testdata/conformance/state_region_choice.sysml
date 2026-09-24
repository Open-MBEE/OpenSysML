// A choice reached from inside an orthogonal region whose branch stays in that
// region moves only that region; its sibling keeps the state it was in.
package Test {
	state def RegionChoice parallel {
		attribute mode : Integer = 2;

		state left {
			entry; then lstart;
			state lstart;
			state lidle;
			state lfast;
			state lslow;

			succession first lstart then lidle;
			transition first lidle then pick;
		}

		state right {
			entry; then rstart;
			state rstart;
			state rwatch;

			succession first rstart then rwatch;
		}

		choice pick;

		transition first pick if mode == 2 then lfast;
		transition first pick then lslow;
	}
}
