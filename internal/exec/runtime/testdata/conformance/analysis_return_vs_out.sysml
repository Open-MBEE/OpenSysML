// An unnamed `return` binds the case's result; a named one (`return r`) is an
// output by that name, which is also the result. `out` parameters are outputs
// beside it.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis unnamed {
		subject s = ship;
		out doubled : Real = s.cost * 2.0;
		return : Real = s.cost * 4.0;
	}

	analysis named {
		subject s = ship;
		return r : Real = s.cost * 3.0;
	}

	attribute namedRead = named.r;
}
