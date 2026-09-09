package ParallelTimed {
	private import SI::*;

	state def Machine parallel {
		state left {
			entry; then waiting;
			state waiting;
			accept after 5 [s] then ldone;
			state ldone;
		}
		state right {
			entry; then active;
			state active;
		}
	}
}
