// A normal has no greatest value: a run that calls it under the max draw policy
// stops with an error naming the call and the policy.
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;

	action unbounded {
		attribute g : Real = normal(12.0, 3.0);

		first start;
		then done;
	}
}
