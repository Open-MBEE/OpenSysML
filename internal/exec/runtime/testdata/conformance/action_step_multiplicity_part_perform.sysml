package test {
	action def Act { }

	part def Host {
		perform action run[2] : Act;
	}
}
