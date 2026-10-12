package EntryTransition {
	action def Mark;

	state def Machine {
		entry action boot { }
		transition first boot do action mark : Mark then active;
		state active;
	}
}
