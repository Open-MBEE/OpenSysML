package test {
	private import ScalarValues::*;

	// A constraint body's local is one binding under its name and short name: its
	// steps write it and its conditions read it through either.
	part def Vehicle {
		constraint shortThenShort { attribute <s> y : Integer = 0; assign s := 5; s == 5 }
		constraint shortThenName { attribute <s> y : Integer = 0; assign s := 5; y == 5 }
		constraint nameThenShort { attribute <s> y : Integer = 0; assign y := 5; s == 5 }
		constraint unvalued { attribute <s> y : Integer; assign s := 5; s == 5 and y == 5 }
	}
	part vehicle : Vehicle;
}
