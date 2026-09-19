// A parameter whose multiplicity admits no value may be left without an
// argument; it is then null, which `??` tells apart from a given value.
package test {
	private import ScalarValues::*;

	calc def Scale {
		in x : Integer;
		in factor : Integer[0..1];
		return : Integer = x * (factor ?? 1);
	}

	calc def Scaling {
		out omitted = Scale(5);
		out given = Scale(5, 3);
	}

	calc c : Scaling;
}
