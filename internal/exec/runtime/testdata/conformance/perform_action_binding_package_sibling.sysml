// A performed action's binding names a package-level part: the name evaluates to
// the object that part is, so the action reads its features.
package test {
	private import ScalarValues::*;

	part def Counter {
		attribute count : Integer = 4;
	}

	action def Bump {
		in c : Counter;
		out n : Integer = c.count + 1;
	}

	part cc : Counter;

	part machine {
		perform action p : Bump { in c = cc; }
	}

	calc def probe {
		return : Integer = machine.p.n;
	}
}
