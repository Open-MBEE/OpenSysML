// Every RandomFunctions call resolved at its max under the max draw policy: a
// uniform at its upper bound, a uniformInteger at its greatest value, a triangular
// at its upper bound, and a timer drawn from uniform(1, 80) [s] waits 80 [s]. No
// seed is set: a fixed policy draws without one.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import RandomFunctions::*;

	action wait {
		attribute d : Real = uniform(0.0, 1.0);
		attribute n : Integer = uniformInteger(1, 6);
		attribute t : Real = triangular(1.0, 4.0, 10.0);

		first start;
		then action wait accept after uniform(1, 80) [s];
		then done;
	}
}
