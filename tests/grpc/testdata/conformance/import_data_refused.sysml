package V {
	private import ScalarValues::*;
	part def Vehicle {
		attribute count : Integer default 1;
		attribute label : String default "plain";
	}
	part vehicle : Vehicle;
	part car : Vehicle {
		attribute :>> count = 2;
	}
}
