package V {
	private import ScalarValues::*;
	part def Vehicle {
		attribute count : Integer default 1;
		attribute pair : Integer[2];
	}
	part vehicle : Vehicle;
}
