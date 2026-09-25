state ParallelSplit {
	entry; then start;
	state start;
	state ready;
	state working parallel {
		state left {
			entry; then leftStart;
			state leftStart;
			state building;
			succession first leftStart then building;
		}
		state right {
			entry; then rightStart;
			state rightStart;
			state checking;
			succession first rightStart then checking;
		}
	}
	fork split;
	join sync;

	succession first start then ready;
	transition first ready then split;
	transition first split then building;
	transition first split then checking;
	transition first building then sync;
	transition first checking then sync;
	transition first sync then done;
}
