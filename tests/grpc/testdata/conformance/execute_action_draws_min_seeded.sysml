package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	action draw {
		out d : Real = uniform(0.0, 1.0);
		out n : Integer = uniformInteger(1, 6);
	}
}
