package test {
	state Machine {
		entry action boot { }
		transition boot then pick;
		junction pick;
		transition first pick then stop;
		action stop terminate;
	}
}
