// A case's actors bind by name only, and so may its inputs: with the actor
// declared first, a named argument binds the input it names and the other input
// keeps its default.
package test {
	private import ScalarValues::*;

	part def Operator;
	part def Thing {
		attribute mass : Real = 4.0;
	}

	analysis def ScaledMass {
		subject t : Thing;
		actor op : Operator;
		in factor : Real;
		in offset : Real default 0.0;
		return : Real = t.mass * factor + offset;
	}

	part thing : Thing;
}
