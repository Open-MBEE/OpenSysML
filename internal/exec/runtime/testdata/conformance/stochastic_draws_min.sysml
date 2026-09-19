// Every RandomFunctions call resolved at its min under the min draw policy, with
// no seed set: the lower bound of a uniform and a triangular, the least value of
// a uniformInteger.
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;

	action least {
		attribute d : Real = uniform(2.5, 7.5);
		attribute n : Integer = uniformInteger(3, 9);
		attribute t : Real = triangular(1.0, 4.0, 10.0);

		first start;
		then done;
	}
}
