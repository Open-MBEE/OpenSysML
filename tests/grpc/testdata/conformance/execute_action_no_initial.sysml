package test {
	action def CycleAction {
		action a;
		action b;
		first a then b;
		first b then a;
	}
}
