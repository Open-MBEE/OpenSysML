// A sub-analysis nested in a case body binds no subject of its own: it runs on
// the enclosing case's subject by default (SysML v2 §7.22), and the enclosing
// case reads its output as a feature of it.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis def Mass {
		subject s : Ship;
		out m : Real = s.cost;
	}

	analysis outer {
		subject s = ship;
		analysis inner : Mass;
		out total : Real = inner.m * 2.0;
	}
}
