package test {
	state def Machine {
		entry; then active;
		state active {
			do action tick[2] { }
			do action sibling { }
		}
	}
}
