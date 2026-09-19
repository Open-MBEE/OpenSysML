// Every RandomFunctions call resolved at its mean under the average draw policy,
// with no seed set: the midpoint of a uniform, the mean (lo + mode + hi) / 3 of a
// triangular, the mean of a normal, and the midpoint of a uniformInteger rounded
// with a half rounded toward hi.
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;

	action mean {
		attribute d : Real = uniform(2.0, 7.0);
		attribute n : Integer = uniformInteger(1, 6);
		attribute t : Real = triangular(1.0, 4.0, 10.0);
		attribute g : Real = normal(12.0, 3.0);

		first start;
		then done;
	}
}
