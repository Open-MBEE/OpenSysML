package test {
	state Machine {
		entry; then init;
		state init;
		state Running;

		succession first init then Running;
		succession first Running then done;
	}
}
