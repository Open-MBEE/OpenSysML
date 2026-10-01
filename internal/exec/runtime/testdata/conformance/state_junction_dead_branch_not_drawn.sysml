package Test {
	attribute def Start;

	state Machine {
		entry; then wait;
		state wait;
		state live;
		state unused;
		junction route;
		junction dead;

		transition first wait accept Start then route;
		transition first route then dead;
		transition first route then live;
		transition first dead if false then unused;
	}
}
