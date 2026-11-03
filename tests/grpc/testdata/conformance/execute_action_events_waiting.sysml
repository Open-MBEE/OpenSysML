package test {
	private import ScalarValues::*;
	item def Go;
	part def Craft;
	part craft : Craft;
	action def Flow {
		in budget : Integer;
		attribute remaining : Integer = budget;
		out landed : Boolean = false;
		first start;
		then action burn assign remaining := remaining - 3;
		then decide route;
			if remaining >= 5 then waiting;
			else finish;
		action waiting accept go : Go;
		then action land assign landed := true;
		then finish;
		action finish;
		then done;
	}
}
