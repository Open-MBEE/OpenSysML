package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import RandomFunctions::*;

	// Random values and a random duration under a fixed model seed: each call draws
	// once from the modeled stream, and the wait draws its duration when it parks.
	action wait {
		attribute d : Real = uniform(0.0, 1.0);
		attribute n : Integer = uniformInteger(1, 6);

		first start;
		then accept after uniform(1, 80) [s];
		then done;
	}
}
