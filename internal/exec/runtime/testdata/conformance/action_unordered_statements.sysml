package test {
	private import ScalarValues::*;

	// An `if`, a `while` and a `for` written among an action's members are its
	// subactions: unordered, each performed once during the action.
	action host {
		attribute c : Integer := 0;
		attribute d : Integer := 0;
		attribute e : Integer := 0;
		if true { assign d := 1; }
		while e < 3 { assign e := e + 1; }
		for i in 1..3 { assign c := c + i; }
	}
}
