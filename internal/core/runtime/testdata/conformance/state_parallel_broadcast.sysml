package ParallelBroadcast {
	item def Tick;

	state def Machine parallel {
		state left {
			entry; then lidle;
			state lidle;
			accept Tick then ldone;
			state ldone;
		}
		state right {
			entry; then ridle;
			state ridle;
			accept Tick then rdone;
			state rdone;
		}
	}
}
