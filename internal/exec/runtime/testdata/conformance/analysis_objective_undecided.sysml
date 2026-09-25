// An objective whose condition cannot be evaluated — it reads a limit no value is
// given for — is undecided rather than an error of the run: the case's outputs
// are still answered.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis undecided {
		subject s = ship;
		attribute limit : Real;
		objective bounded {
			require constraint { c < limit }
		}
		out c : Real = s.cost;
	}
}
